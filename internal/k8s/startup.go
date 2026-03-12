package k8s

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodStartupInfo holds the timing breakdown of a pod's startup sequence.
type PodStartupInfo struct {
	PodName   string
	Namespace string
	TotalTime time.Duration
	Phases    []StartupPhase
}

// StartupPhase represents a single phase in the pod startup sequence.
type StartupPhase struct {
	Name     string
	Duration time.Duration
	Status   string // "completed", "in-progress", "unknown"
}

// GetPodStartupAnalysis fetches a pod and its events to compute a startup timing breakdown.
func (c *Client) GetPodStartupAnalysis(ctx context.Context, contextName, namespace, podName string) (*PodStartupInfo, error) {
	clientset, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}

	pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting pod: %w", err)
	}

	info := &PodStartupInfo{
		PodName:   podName,
		Namespace: namespace,
	}

	creationTime := pod.CreationTimestamp.Time
	now := time.Now()

	// Extract condition timestamps.
	var scheduledTime, initializedTime, containersReadyTime, readyTime time.Time
	for _, cond := range pod.Status.Conditions {
		if cond.LastTransitionTime.IsZero() {
			continue
		}
		switch cond.Type {
		case "PodScheduled":
			scheduledTime = cond.LastTransitionTime.Time
		case "Initialized":
			initializedTime = cond.LastTransitionTime.Time
		case "ContainersReady":
			containersReadyTime = cond.LastTransitionTime.Time
		case "Ready":
			readyTime = cond.LastTransitionTime.Time
		}
	}

	// Phase 1: Scheduling (Created -> PodScheduled).
	if !scheduledTime.IsZero() {
		info.Phases = append(info.Phases, StartupPhase{
			Name:     "Scheduling",
			Duration: scheduledTime.Sub(creationTime),
			Status:   "completed",
		})
	} else {
		info.Phases = append(info.Phases, StartupPhase{
			Name:     "Scheduling",
			Duration: now.Sub(creationTime),
			Status:   "in-progress",
		})
	}

	// Phase 2: Image Pull (from events).
	events, err := clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod", podName),
	})
	if err == nil && events != nil {
		pullDuration := computeImagePullTime(events.Items)
		if pullDuration > 0 {
			info.Phases = append(info.Phases, StartupPhase{
				Name:     "Image Pull",
				Duration: pullDuration,
				Status:   "completed",
			})
		} else {
			// Check if pulling is in progress.
			for _, ev := range events.Items {
				if ev.Reason == "Pulling" {
					info.Phases = append(info.Phases, StartupPhase{
						Name:     "Image Pull",
						Duration: now.Sub(ev.LastTimestamp.Time),
						Status:   "in-progress",
					})
					break
				}
			}
		}
	}

	// Phase 3: Init Containers (PodScheduled -> Initialized).
	hasInitContainers := len(pod.Spec.InitContainers) > 0
	if hasInitContainers {
		if !initializedTime.IsZero() && !scheduledTime.IsZero() {
			info.Phases = append(info.Phases, StartupPhase{
				Name:     "Init Containers",
				Duration: initializedTime.Sub(scheduledTime),
				Status:   "completed",
			})
		} else if !scheduledTime.IsZero() {
			info.Phases = append(info.Phases, StartupPhase{
				Name:     "Init Containers",
				Duration: now.Sub(scheduledTime),
				Status:   "in-progress",
			})
		}

		// Add per-init-container timing if available.
		for _, cs := range pod.Status.InitContainerStatuses {
			if cs.State.Terminated != nil {
				start := cs.State.Terminated.StartedAt.Time
				finish := cs.State.Terminated.FinishedAt.Time
				if !start.IsZero() && !finish.IsZero() {
					info.Phases = append(info.Phases, StartupPhase{
						Name:     fmt.Sprintf("  init: %s", cs.Name),
						Duration: finish.Sub(start),
						Status:   "completed",
					})
				}
			} else if cs.State.Running != nil {
				info.Phases = append(info.Phases, StartupPhase{
					Name:     fmt.Sprintf("  init: %s", cs.Name),
					Duration: now.Sub(cs.State.Running.StartedAt.Time),
					Status:   "in-progress",
				})
			} else {
				info.Phases = append(info.Phases, StartupPhase{
					Name:     fmt.Sprintf("  init: %s", cs.Name),
					Duration: 0,
					Status:   "unknown",
				})
			}
		}
	}

	// Phase 4: Container Startup (Initialized -> ContainersReady).
	baseTime := initializedTime
	if baseTime.IsZero() {
		baseTime = scheduledTime
	}
	if !containersReadyTime.IsZero() && !baseTime.IsZero() {
		info.Phases = append(info.Phases, StartupPhase{
			Name:     "Container Startup",
			Duration: containersReadyTime.Sub(baseTime),
			Status:   "completed",
		})
	} else if !baseTime.IsZero() {
		info.Phases = append(info.Phases, StartupPhase{
			Name:     "Container Startup",
			Duration: now.Sub(baseTime),
			Status:   "in-progress",
		})
	}

	// Add per-container timing.
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Running != nil {
			startedAt := cs.State.Running.StartedAt.Time
			if !startedAt.IsZero() && !containersReadyTime.IsZero() {
				info.Phases = append(info.Phases, StartupPhase{
					Name:     fmt.Sprintf("  container: %s", cs.Name),
					Duration: containersReadyTime.Sub(startedAt),
					Status:   "completed",
				})
			} else if !startedAt.IsZero() {
				info.Phases = append(info.Phases, StartupPhase{
					Name:     fmt.Sprintf("  container: %s", cs.Name),
					Duration: now.Sub(startedAt),
					Status:   "in-progress",
				})
			}
		} else if cs.State.Terminated != nil && !cs.State.Terminated.StartedAt.IsZero() {
			start := cs.State.Terminated.StartedAt.Time
			finish := cs.State.Terminated.FinishedAt.Time
			if !finish.IsZero() {
				info.Phases = append(info.Phases, StartupPhase{
					Name:     fmt.Sprintf("  container: %s", cs.Name),
					Duration: finish.Sub(start),
					Status:   "completed",
				})
			}
		} else {
			info.Phases = append(info.Phases, StartupPhase{
				Name:     fmt.Sprintf("  container: %s", cs.Name),
				Duration: 0,
				Status:   "unknown",
			})
		}
	}

	// Phase 5: Readiness (ContainersReady -> Ready).
	if !readyTime.IsZero() && !containersReadyTime.IsZero() {
		readinessDur := readyTime.Sub(containersReadyTime)
		if readinessDur > 0 {
			info.Phases = append(info.Phases, StartupPhase{
				Name:     "Readiness Probes",
				Duration: readinessDur,
				Status:   "completed",
			})
		}
	} else if !containersReadyTime.IsZero() && readyTime.IsZero() {
		info.Phases = append(info.Phases, StartupPhase{
			Name:     "Readiness Probes",
			Duration: now.Sub(containersReadyTime),
			Status:   "in-progress",
		})
	}

	// Compute total time.
	if !readyTime.IsZero() {
		info.TotalTime = readyTime.Sub(creationTime)
	} else if !containersReadyTime.IsZero() {
		info.TotalTime = containersReadyTime.Sub(creationTime)
	} else {
		info.TotalTime = now.Sub(creationTime)
	}

	return info, nil
}

// computeImagePullTime calculates total image pull duration from events.
// It pairs "Pulling" and "Pulled" events for each image and sums up the durations.
func computeImagePullTime(events []corev1.Event) time.Duration {
	type pullPair struct {
		pulling time.Time
		pulled  time.Time
	}

	// Group by image name (extracted from the event message).
	pulls := make(map[string]*pullPair)

	// Sort events by timestamp to process them in order.
	sort.Slice(events, func(i, j int) bool {
		return events[i].LastTimestamp.Time.Before(events[j].LastTimestamp.Time)
	})

	for _, ev := range events {
		switch ev.Reason {
		case "Pulling":
			image := extractImageFromMessage(ev.Message)
			if image != "" {
				if _, ok := pulls[image]; !ok {
					pulls[image] = &pullPair{}
				}
				pulls[image].pulling = ev.LastTimestamp.Time
			}
		case "Pulled":
			image := extractImageFromMessage(ev.Message)
			if image != "" {
				if _, ok := pulls[image]; !ok {
					pulls[image] = &pullPair{}
				}
				pulls[image].pulled = ev.LastTimestamp.Time
			}
		}
	}

	var total time.Duration
	for _, pair := range pulls {
		if !pair.pulling.IsZero() && !pair.pulled.IsZero() {
			d := pair.pulled.Sub(pair.pulling)
			if d > 0 {
				total += d
			}
		}
	}
	return total
}

// extractImageFromMessage extracts an image name from a Pulling/Pulled event message.
// Typical formats: "Pulling image \"nginx:latest\"" or "Successfully pulled image \"nginx:latest\""
func extractImageFromMessage(message string) string {
	// Look for content between quotes.
	start := strings.Index(message, "\"")
	if start < 0 {
		return ""
	}
	end := strings.Index(message[start+1:], "\"")
	if end < 0 {
		return ""
	}
	return message[start+1 : start+1+end]
}
