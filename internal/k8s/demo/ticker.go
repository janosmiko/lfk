package demo

import (
	"context"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// tickerFieldManager marks writes the Ticker makes through the dynamic
// client, distinguishing them from the initial seed data in managedFields.
const tickerFieldManager = "lfk-demo-ticker"

// maxTickerEvents caps how many Warning Events the ticker keeps appending
// for the crashlooping pod. A demo session left running ticks indefinitely;
// without a cap the Event list would grow without bound.
const maxTickerEvents = 20

// healthyPodFlipEvery is how many ticks pass between phase flips on
// PodWebHealthy2.
const healthyPodFlipEvery = 5

var (
	podGVR        = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	eventGVR      = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "events"}
	deploymentGVR = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
)

// waitingReasons is the sequence of waiting-container reasons the
// crashlooping pod cycles through on each restart tick.
var waitingReasons = []string{"CrashLoopBackOff", "Error"}

// deploymentReadyReplicaCycle is the sequence of readyReplicas values the
// web Deployment cycles through, out of its 3 desired replicas.
var deploymentReadyReplicaCycle = []int64{2, 1, 3}

// Ticker mutates a demo cluster's fake dynamic client on a fixed interval so
// its informer-backed watchers see live changes: the crashlooping pod
// restarts and its waiting reason rotates, a matching Warning Event lands, a
// healthy pod's phase flips occasionally, and the web Deployment's
// readyReplicas drifts. Every mutation is a deterministic function of the
// tick count, so driving a fresh Ticker N times always reaches the same
// state — tests and screenshots stay stable.
//
// Ticker only touches the dynamic client passed to NewTicker; it is not
// wired into any client lifecycle here.
type Ticker struct {
	dyn      dynamic.Interface
	interval time.Duration

	mu            sync.Mutex
	tick          uint64
	running       bool
	cancel        context.CancelFunc
	done          chan struct{}
	restartEvents []string // FIFO of ticker-created event names, oldest first
}

// NewTicker returns a Ticker over dyn that, once started, mutates the demo
// cluster every interval. It does not touch dyn until Start or Tick is
// called.
func NewTicker(dyn dynamic.Interface, interval time.Duration) *Ticker {
	return &Ticker{dyn: dyn, interval: interval}
}

// Start launches the ticker's goroutine, which calls Tick every interval
// until Stop is called or ctx is cancelled. Calling Start while already
// running is a no-op.
func (t *Ticker) Start(ctx context.Context) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.running {
		return
	}

	runCtx, cancel := context.WithCancel(ctx)
	t.cancel = cancel
	t.done = make(chan struct{})
	t.running = true

	go t.run(runCtx, t.done)
}

func (t *Ticker) run(ctx context.Context, done chan struct{}) {
	defer close(done)

	// time.NewTicker panics for a non-positive duration; treat a
	// misconfigured interval as a no-op run instead of crashing the
	// process from a background goroutine.
	if t.interval <= 0 {
		return
	}

	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Best-effort: a single failed mutation should not take down
			// the background loop. The caller wiring this in decides
			// whether failures need surfacing.
			_ = t.Tick(ctx)
		}
	}
}

// Stop cancels the ticker's goroutine and blocks until it has exited.
// Idempotent and safe to call even if Start was never called; concurrent
// Stop calls all wait for the same goroutine to exit before any of them
// return. Start and Stop are not safe to call concurrently with each
// other — serialize lifecycle calls at the call site.
func (t *Ticker) Stop() {
	t.mu.Lock()
	if !t.running {
		t.mu.Unlock()
		return
	}
	cancel := t.cancel
	done := t.done
	t.mu.Unlock()

	cancel()
	<-done

	t.mu.Lock()
	t.running = false
	t.mu.Unlock()
}

// Tick performs one deterministic mutation step against the tracked demo
// objects. Exported so tests can drive the sequence precisely without
// sleeping on real time; Start's internal loop calls it the same way. The
// whole step runs under the Ticker's lock so a direct Tick call never races
// against Start's background loop calling it concurrently.
func (t *Ticker) Tick(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	tick := t.tick
	t.tick++

	if err := t.restartCrashLoopPod(ctx, tick); err != nil {
		return fmt.Errorf("demo ticker: restarting crashloop pod: %w", err)
	}
	if err := t.appendRestartEvent(ctx, tick); err != nil {
		return fmt.Errorf("demo ticker: appending restart event: %w", err)
	}
	if err := t.flipHealthyPodPhase(ctx, tick); err != nil {
		return fmt.Errorf("demo ticker: flipping healthy pod phase: %w", err)
	}
	if err := t.driftDeploymentReadyReplicas(ctx, tick); err != nil {
		return fmt.Errorf("demo ticker: drifting deployment ready replicas: %w", err)
	}
	return nil
}

// restartCrashLoopPod increments PodWebCrashLoop's restartCount and rotates
// its waiting reason through waitingReasons.
func (t *Ticker) restartCrashLoopPod(ctx context.Context, tick uint64) error {
	client := t.dyn.Resource(podGVR).Namespace(NamespaceDemo)
	obj, err := client.Get(ctx, PodWebCrashLoop, metav1.GetOptions{})
	if err != nil {
		return err
	}

	statuses, found, err := unstructured.NestedSlice(obj.Object, "status", "containerStatuses")
	if err != nil {
		return err
	}
	if !found || len(statuses) == 0 {
		return fmt.Errorf("pod %s has no containerStatuses", PodWebCrashLoop)
	}
	cs, ok := statuses[0].(map[string]any)
	if !ok {
		return fmt.Errorf("pod %s containerStatuses[0] has an unexpected shape", PodWebCrashLoop)
	}

	restartCount, _, err := unstructured.NestedInt64(cs, "restartCount")
	if err != nil {
		return err
	}
	if err := unstructured.SetNestedField(cs, restartCount+1, "restartCount"); err != nil {
		return err
	}

	reason := waitingReasons[tick%uint64(len(waitingReasons))]
	if err := unstructured.SetNestedField(cs, reason, "state", "waiting", "reason"); err != nil {
		return err
	}
	message := fmt.Sprintf("back-off restarting failed container=web pod=%s_%s(%s)",
		PodWebCrashLoop, NamespaceDemo, uidPodWebCrash)
	if err := unstructured.SetNestedField(cs, message, "state", "waiting", "message"); err != nil {
		return err
	}

	statuses[0] = cs
	if err := unstructured.SetNestedSlice(obj.Object, statuses, "status", "containerStatuses"); err != nil {
		return err
	}

	_, err = client.Update(ctx, obj, metav1.UpdateOptions{FieldManager: tickerFieldManager})
	return err
}

// appendRestartEvent creates a fresh Warning Event for the crashlooping
// pod's restart, then evicts the oldest ticker-created event once the count
// exceeds maxTickerEvents.
func (t *Ticker) appendRestartEvent(ctx context.Context, tick uint64) error {
	client := t.dyn.Resource(eventGVR).Namespace(NamespaceDemo)

	name := fmt.Sprintf("web-crash-restart-%d", tick)
	uid := fmt.Sprintf("e0000000-0000-4000-8000-%012d", tick)
	now := time.Now().UTC()
	message := fmt.Sprintf("Back-off restarting failed container web in pod %s_%s(%s)",
		PodWebCrashLoop, NamespaceDemo, uidPodWebCrash)
	evt := podWarningEvent(name, uid, PodWebCrashLoop, uidPodWebCrash, "BackOff", message, 1, now, now)

	if _, err := client.Create(ctx, mustToUnstructured(evt), metav1.CreateOptions{FieldManager: tickerFieldManager}); err != nil {
		return err
	}

	// Tick already holds t.mu for the duration of this call, so the FIFO
	// update below needs no locking of its own.
	t.restartEvents = append(t.restartEvents, name)
	var evict string
	if len(t.restartEvents) > maxTickerEvents {
		evict, t.restartEvents = t.restartEvents[0], t.restartEvents[1:]
	}

	if evict == "" {
		return nil
	}
	if err := client.Delete(ctx, evict, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("evicting event %s: %w", evict, err)
	}
	return nil
}

// flipHealthyPodPhase toggles PodWebHealthy2 between Running and Pending
// every healthyPodFlipEvery ticks, keeping its Ready condition consistent
// with the phase.
func (t *Ticker) flipHealthyPodPhase(ctx context.Context, tick uint64) error {
	client := t.dyn.Resource(podGVR).Namespace(NamespaceDemo)
	obj, err := client.Get(ctx, PodWebHealthy2, metav1.GetOptions{})
	if err != nil {
		return err
	}

	phase := string(corev1.PodRunning)
	condStatus := string(corev1.ConditionTrue)
	if (tick/healthyPodFlipEvery)%2 == 1 {
		phase = string(corev1.PodPending)
		condStatus = string(corev1.ConditionFalse)
	}

	if err := unstructured.SetNestedField(obj.Object, phase, "status", "phase"); err != nil {
		return err
	}

	conditions, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil {
		return err
	}
	if found && len(conditions) > 0 {
		c0, ok := conditions[0].(map[string]any)
		if !ok {
			return fmt.Errorf("pod %s conditions[0] has an unexpected shape", PodWebHealthy2)
		}
		if err := unstructured.SetNestedField(c0, condStatus, "status"); err != nil {
			return err
		}
		conditions[0] = c0
		if err := unstructured.SetNestedSlice(obj.Object, conditions, "status", "conditions"); err != nil {
			return err
		}
	}

	_, err = client.Update(ctx, obj, metav1.UpdateOptions{FieldManager: tickerFieldManager})
	return err
}

// driftDeploymentReadyReplicas cycles DeploymentWeb's readyReplicas (and
// availableReplicas alongside it) through deploymentReadyReplicaCycle.
func (t *Ticker) driftDeploymentReadyReplicas(ctx context.Context, tick uint64) error {
	client := t.dyn.Resource(deploymentGVR).Namespace(NamespaceDemo)
	obj, err := client.Get(ctx, DeploymentWeb, metav1.GetOptions{})
	if err != nil {
		return err
	}

	ready := deploymentReadyReplicaCycle[tick%uint64(len(deploymentReadyReplicaCycle))]
	if err := unstructured.SetNestedField(obj.Object, ready, "status", "readyReplicas"); err != nil {
		return err
	}
	if err := unstructured.SetNestedField(obj.Object, ready, "status", "availableReplicas"); err != nil {
		return err
	}

	_, err = client.Update(ctx, obj, metav1.UpdateOptions{FieldManager: tickerFieldManager})
	return err
}
