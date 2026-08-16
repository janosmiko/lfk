package k8s

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"

	"golang.org/x/sync/errgroup"
)

// CrashInvestigation is the full result of GetCrashInvestigation: pod
// summary, per-container restart + termination + log info, pod-scoped
// events, and a (possibly trimmed) describe blob.
type CrashInvestigation struct {
	Pod            PodSummary
	InitContainers []ContainerCrash // declaration order. Init containers
	AppContainers  []ContainerCrash // declaration order. App containers
	Events         []corev1.Event   // sorted by LastTimestamp desc
	Describe       string
	DescribeError  string
}

// PodSummary holds pod-scoped fields rendered in the overlay header
// and the Summary tab. Owner refs are flattened to a single
// (Kind, Name) pair (the controller ref).
type PodSummary struct {
	Name      string
	Namespace string
	Phase     string
	PodIP     string
	Node      string
	QoSClass  string
	Age       time.Duration
	OwnerKind string
	OwnerName string
}

// ContainerCrash captures a single container's runtime state plus its
// previous-instance and current-instance log tails.
type ContainerCrash struct {
	Name            string
	IsInit          bool
	Image           string
	State           string // "Running", "Waiting", "Terminated"
	StateReason     string // "CrashLoopBackOff", "Error", etc.
	Ready           bool
	RestartCount    int32
	Started         *time.Time
	LastTermination *ContainerTermination

	PreviousLog string
	CurrentLog  string
	LogError    string // joined per-stream errors (previous + current)
}

// ContainerTermination is the LastTerminationState.Terminated info,
// extracted to a flat struct so the renderer doesn't depend on
// k8s.io/api/core/v1.
type ContainerTermination struct {
	Reason     string
	ExitCode   int32
	Signal     int32
	StartedAt  time.Time
	FinishedAt time.Time
	Message    string
}

// crashLogTailLines is the number of trailing log lines fetched per
// container instance (previous + current) by GetCrashInvestigation.
const crashLogTailLines int64 = 200

// crashLogStreamConcurrency caps the number of in-flight log fetches in
// fetchContainerLogs. Each container needs 2 streams (previous + current),
// so 8 covers a 4-container pod fully and throttles fan-out for larger pods.
const crashLogStreamConcurrency = 8

// GetCrashInvestigation fetches a pod and assembles a multi-section
// diagnostic snapshot: pod summary, per-container restart + termination
// + log info, pod-scoped events, and a kubectl describe blob. Events are
// scoped to the current pod incarnation by UID so a recreated pod with the
// same name does not surface old-incarnation events. Per-stream log errors
// and a describe-fetch error do NOT fail the whole call. Only a Pod Get
// failure does.
func (c *Client) GetCrashInvestigation(ctx context.Context, contextName, namespace, podName string) (*CrashInvestigation, error) {
	clientset, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, fmt.Errorf("clientset: %w", err)
	}

	pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("pod %s/%s not found: %w", namespace, podName, err)
		}
		return nil, fmt.Errorf("getting pod: %w", err)
	}

	out := &CrashInvestigation{
		Pod: buildPodSummary(pod),
	}
	out.InitContainers, out.AppContainers = buildContainerCrashes(pod)
	out.Events = fetchPodEvents(ctx, clientset, namespace, podName, string(pod.UID))
	out.InitContainers = fetchContainerLogs(ctx, clientset, namespace, podName, out.InitContainers)
	out.AppContainers = fetchContainerLogs(ctx, clientset, namespace, podName, out.AppContainers)

	// Describe (errors don't fail the whole call).
	desc, derr := c.describeForCrashInvestigator(ctx, contextName, namespace, podName)
	if derr != nil {
		out.DescribeError = derr.Error()
	} else {
		out.Describe = desc
	}

	return out, nil
}

// describeForCrashInvestigator routes describe lookups through the test
// override when set, otherwise delegates to the production DescribePod
// path (which shells out to kubectl).
func (c *Client) describeForCrashInvestigator(ctx context.Context, contextName, namespace, podName string) (string, error) {
	if c.describeOverride != nil {
		return c.describeOverride(ctx, contextName, namespace, podName)
	}
	return c.DescribePod(ctx, contextName, namespace, podName)
}

// buildPodSummary maps the pod-scoped fields of a corev1.Pod into the
// flat PodSummary struct, picking the controller OwnerReference (if any)
// for OwnerKind/OwnerName.
func buildPodSummary(pod *corev1.Pod) PodSummary {
	s := PodSummary{
		Name:      pod.Name,
		Namespace: pod.Namespace,
		Phase:     string(pod.Status.Phase),
		PodIP:     pod.Status.PodIP,
		Node:      pod.Spec.NodeName,
		QoSClass:  string(pod.Status.QOSClass),
		Age:       time.Since(pod.CreationTimestamp.Time),
	}
	for _, ref := range pod.OwnerReferences {
		if ref.Controller != nil && *ref.Controller {
			s.OwnerKind = ref.Kind
			s.OwnerName = ref.Name
			break
		}
	}
	return s
}

// buildContainerCrashes returns init + app ContainerCrash slices in the
// pod's declaration order. Statuses are matched by name. Spec containers
// without a matching status get a zero-value entry (e.g. pods scheduled
// but the kubelet hasn't reported yet).
func buildContainerCrashes(pod *corev1.Pod) (initCC, appCC []ContainerCrash) {
	initStatuses := indexContainerStatuses(pod.Status.InitContainerStatuses)
	appStatuses := indexContainerStatuses(pod.Status.ContainerStatuses)

	initCC = make([]ContainerCrash, 0, len(pod.Spec.InitContainers))
	for _, c := range pod.Spec.InitContainers {
		initCC = append(initCC, buildContainerCrash(c, initStatuses[c.Name], true))
	}
	appCC = make([]ContainerCrash, 0, len(pod.Spec.Containers))
	for _, c := range pod.Spec.Containers {
		appCC = append(appCC, buildContainerCrash(c, appStatuses[c.Name], false))
	}
	return initCC, appCC
}

// indexContainerStatuses returns a name -> *ContainerStatus map so callers
// can look up a status by container name in O(1) without copying the
// underlying status struct.
func indexContainerStatuses(statuses []corev1.ContainerStatus) map[string]*corev1.ContainerStatus {
	out := make(map[string]*corev1.ContainerStatus, len(statuses))
	for i := range statuses {
		out[statuses[i].Name] = &statuses[i]
	}
	return out
}

// buildContainerCrash maps a single container spec + (possibly nil) status
// into a ContainerCrash. A nil status means the kubelet hasn't reported a
// state yet, which we surface as "Waiting" with no reason.
func buildContainerCrash(spec corev1.Container, status *corev1.ContainerStatus, isInit bool) ContainerCrash {
	cc := ContainerCrash{
		Name:   spec.Name,
		Image:  spec.Image,
		IsInit: isInit,
	}
	if status == nil {
		cc.State = "Waiting"
		return cc
	}
	cc.Ready = status.Ready
	cc.RestartCount = status.RestartCount
	switch {
	case status.State.Running != nil:
		cc.State = "Running"
		t := status.State.Running.StartedAt.Time
		cc.Started = &t
	case status.State.Terminated != nil:
		cc.State = "Terminated"
		cc.StateReason = status.State.Terminated.Reason
	case status.State.Waiting != nil:
		cc.State = "Waiting"
		cc.StateReason = status.State.Waiting.Reason
	default:
		cc.State = "Waiting"
	}
	if t := status.LastTerminationState.Terminated; t != nil {
		cc.LastTermination = &ContainerTermination{
			Reason:     t.Reason,
			ExitCode:   t.ExitCode,
			Signal:     t.Signal,
			StartedAt:  t.StartedAt.Time,
			FinishedAt: t.FinishedAt.Time,
			Message:    t.Message,
		}
	}
	return cc
}

// fetchPodEvents returns events whose involvedObject points at the given
// pod, sorted by LastTimestamp descending. On a transient list error we
// return an empty slice rather than failing the whole investigation —
// missing events are less harmful than denying the user the rest of the
// diagnostic snapshot.
//
// FieldSelector is a server-side optimization. We still re-filter on the
// client because (a) the fake clientset ignores FieldSelector and would
// otherwise return cross-pod events in tests, and (b) production servers
// occasionally return extra rows when watch-cached.
//
// When podUID is non-empty the selector and client-side filter additionally
// scope events to that pod incarnation. This prevents events from a previous
// pod with the same name (delete + recreate via a controller) from leaking
// into the current investigation. Tests that don't set a UID on the pod pass
// "" and get the legacy name+kind-only behavior.
func fetchPodEvents(ctx context.Context, clientset kubernetes.Interface, namespace, podName, podUID string) []corev1.Event {
	selectorMap := fields.Set{
		"involvedObject.name": podName,
		"involvedObject.kind": "Pod",
	}
	if podUID != "" {
		selectorMap["involvedObject.uid"] = podUID
	}
	selector := fields.SelectorFromSet(selectorMap).String()
	list, err := clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{FieldSelector: selector})
	if err != nil || list == nil {
		return nil
	}
	out := make([]corev1.Event, 0, len(list.Items))
	for _, ev := range list.Items {
		if ev.InvolvedObject.Name != podName || ev.InvolvedObject.Kind != "Pod" {
			continue
		}
		if podUID != "" && string(ev.InvolvedObject.UID) != podUID {
			continue
		}
		out = append(out, ev)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].LastTimestamp.After(out[j].LastTimestamp.Time)
	})
	return out
}

// fetchContainerLogs streams the previous + current tails for every container
// in containers, in parallel. Per-stream errors are stored on the matching
// container as LogError. A stream that returns no content but no error
// (e.g. previous logs not available because container has not been
// terminated yet) leaves both LogError and the corresponding *Log empty.
func fetchContainerLogs(ctx context.Context, clientset kubernetes.Interface, namespace, podName string, containers []ContainerCrash) []ContainerCrash {
	if len(containers) == 0 {
		return containers
	}
	var mu sync.Mutex
	g, gctx := errgroup.WithContext(ctx)
	// Cap concurrent log streams across all containers — large pods (8+
	// containers) shouldn't fan out 16+ HTTP streams against a single API
	// server. Smaller pods are unaffected.
	g.SetLimit(crashLogStreamConcurrency)

	for i := range containers {
		name := containers[i].Name

		// Previous logs.
		g.Go(func() error {
			body, err := getLogTail(gctx, clientset, namespace, podName, name, true)
			mu.Lock()
			defer mu.Unlock()
			if err != nil && !isPreviousLogsUnavailable(err) {
				containers[i].LogError = joinLogErr(containers[i].LogError, fmt.Errorf("previous: %w", err))
				return nil
			}
			containers[i].PreviousLog = body
			return nil
		})

		// Current logs.
		g.Go(func() error {
			body, err := getLogTail(gctx, clientset, namespace, podName, name, false)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				containers[i].LogError = joinLogErr(containers[i].LogError, fmt.Errorf("current: %w", err))
				return nil
			}
			containers[i].CurrentLog = body
			return nil
		})
	}
	_ = g.Wait()
	return containers
}

// getLogTail returns the trailing crashLogTailLines lines of a single
// container's log. If previous is true, it asks the apiserver for the
// previous-instance log (typically only present after a restart).
func getLogTail(ctx context.Context, clientset kubernetes.Interface, namespace, podName, container string, previous bool) (string, error) {
	tail := crashLogTailLines
	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container: container,
		Previous:  previous,
		TailLines: &tail,
	})
	body, err := req.DoRaw(ctx)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// isPreviousLogsUnavailable matches the apiserver's specific responses for
// "no previous instance to read from" so we treat that as expected emptiness
// rather than a real error worth surfacing to the user. Bare "not found" is
// deliberately NOT matched here because it would also swallow a wrong
// container name (`container "xyz" not found`), which is a real
// configuration error the user needs to see.
func isPreviousLogsUnavailable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "previous terminated container") ||
		strings.Contains(msg, "PodInitializing") ||
		strings.Contains(msg, "ContainerCreating")
}

// joinLogErr appends next to prev with a "; " separator, treating empty
// prev as the base case. Used to combine previous + current stream errors
// onto a single ContainerCrash.LogError field without losing either.
func joinLogErr(prev string, next error) string {
	if next == nil {
		return prev
	}
	if prev == "" {
		return next.Error()
	}
	return prev + "; " + next.Error()
}
