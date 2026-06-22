package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/model"
)

// hpaScaleFieldCount is the number of editable rows in the HPA scale overlay
// (min, max, target).
const hpaScaleFieldCount = 3

// active returns a pointer to the currently focused input row.
func (s *hpaScaleState) active() *TextInput {
	switch s.field {
	case 1:
		return &s.max
	case 2:
		return &s.target
	default:
		return &s.min
	}
}

// floor returns the lowest legal value for the active field. minReplicas and
// maxReplicas must be >= 1; the target workload may scale to 0.
func (s *hpaScaleState) floor() int {
	if s.field == 2 {
		return 0
	}
	return 1
}

// nestedInt reads an integer at the given path, tolerating both the float64
// and int64 forms unstructured objects can carry for the same JSON number.
func nestedInt(obj map[string]any, fields ...string) (int64, bool) {
	v, found, err := unstructured.NestedFieldNoCopy(obj, fields...)
	if err != nil || !found {
		return 0, false
	}
	switch n := v.(type) {
	case int64:
		return n, true
	case float64:
		return int64(n), true
	case int:
		return int64(n), true
	}
	return 0, false
}

// openHPAScaleOverlay prefills the HPA scale overlay and opens it. Min/Max come
// from the HPA spec; the target replica field is seeded from the HPA's desired
// (falling back to current) replicas, and the scale target (Kind/Name) from
// spec.scaleTargetRef. Values are read from the raw object, falling back to the
// item's columns when the raw object is unavailable (synthetic items / tests).
func (m Model) openHPAScaleOverlay() Model {
	var minStr, maxStr, targetStr, kind, name string

	if raw := m.actionCtx.raw; raw != nil {
		if v, ok := nestedInt(raw, "spec", "minReplicas"); ok {
			minStr = strconv.FormatInt(v, 10)
		}
		if v, ok := nestedInt(raw, "spec", "maxReplicas"); ok {
			maxStr = strconv.FormatInt(v, 10)
		}
		if v, ok := nestedInt(raw, "status", "desiredReplicas"); ok {
			targetStr = strconv.FormatInt(v, 10)
		} else if v, ok := nestedInt(raw, "status", "currentReplicas"); ok {
			targetStr = strconv.FormatInt(v, 10)
		}
		kind, _, _ = unstructured.NestedString(raw, "spec", "scaleTargetRef", "kind")
		name, _, _ = unstructured.NestedString(raw, "spec", "scaleTargetRef", "name")
	}

	item := model.Item{Columns: m.actionCtx.columns}
	if minStr == "" {
		minStr = columnValue(item, "Min Replicas")
	}
	if maxStr == "" {
		maxStr = columnValue(item, "Max Replicas")
	}
	if targetStr == "" {
		if targetStr = columnValue(item, "Desired Replicas"); targetStr == "" {
			targetStr = columnValue(item, "Current Replicas")
		}
	}
	if kind == "" || name == "" {
		if ref := columnValue(item, "Target"); ref != "" {
			if k, n, ok := strings.Cut(ref, "/"); ok {
				kind, name = k, n
			}
		}
	}

	st := hpaScaleState{targetKind: kind, targetName: name}
	st.min.Set(minStr)
	st.max.Set(maxStr)
	st.target.Set(targetStr)
	st.origMin, st.origMax, st.origTarget = minStr, maxStr, targetStr
	m.hpaScale = st
	m.overlay = overlayHPAScale
	return m
}

// handleHPAScaleOverlayKey routes a keypress in the HPA scale overlay: j/k move
// between fields, h/- and l/+ step the active value, arrows move the cursor,
// digits type, enter applies, esc cancels.
func (m Model) handleHPAScaleOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		m.hpaScale = hpaScaleState{}
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	case "j", "down":
		m.hpaScale.field = (m.hpaScale.field + 1) % hpaScaleFieldCount
		return m, nil
	case "k", "up":
		m.hpaScale.field = (m.hpaScale.field - 1 + hpaScaleFieldCount) % hpaScaleFieldCount
		return m, nil
	case "l", "+":
		stepInput(m.hpaScale.active(), 1, m.hpaScale.floor())
		return m, nil
	case "h", "-":
		stepInput(m.hpaScale.active(), -1, m.hpaScale.floor())
		return m, nil
	case "left":
		m.hpaScale.active().Left()
		return m, nil
	case "right":
		m.hpaScale.active().Right()
		return m, nil
	case "enter":
		return m.applyHPAScaleOverlay()
	case "backspace":
		m.hpaScale.active().Backspace()
		return m, nil
	case "ctrl+w":
		m.hpaScale.active().DeleteWord()
		return m, nil
	default:
		key := msg.String()
		if len(key) == 1 && key[0] >= '0' && key[0] <= '9' {
			m.hpaScale.active().Insert(key)
		}
		return m, nil
	}
}

// stepInput increments (or decrements) a numeric text input by delta, clamping
// at floor. A non-numeric or empty value is treated as floor before stepping.
func stepInput(t *TextInput, delta, floor int) {
	n, err := strconv.Atoi(strings.TrimSpace(t.Value))
	if err != nil {
		n = floor
	}
	n += delta
	if n < floor {
		n = floor
	}
	t.Set(strconv.Itoa(n))
}

// applyHPAScaleOverlay validates the three inputs and dispatches the patch
// (min/max on the HPA) and/or the target scale, for whichever fields changed.
func (m Model) applyHPAScaleOverlay() (tea.Model, tea.Cmd) {
	st := m.hpaScale
	scaleMinMax := st.min.Value != st.origMin || st.max.Value != st.origMax
	scaleTarget := st.target.Value != st.origTarget && st.targetName != ""

	if !scaleMinMax && !scaleTarget {
		m.overlay = overlayNone
		m.hpaScale = hpaScaleState{}
		m.setStatusMessage("No changes", false)
		return m, scheduleStatusClear()
	}

	minR, maxR, targetR, err := parseHPAScaleInputs(st, scaleMinMax, scaleTarget)
	if err != nil {
		m.overlay = overlayNone
		m.hpaScale = hpaScaleState{}
		m.setStatusMessage(err.Error(), true)
		return m, scheduleStatusClear()
	}

	// Belt-and-suspenders read-only gate: the dispatcher blocks "Scale"
	// upstream, but a user who toggled RO on while this overlay was open
	// could otherwise commit the change.
	if m.actionTargetBlockedByReadOnly() {
		m.overlay = overlayNone
		m.hpaScale = hpaScaleState{}
		m.setStatusMessage(readOnlyBlockedMessage("Scale"), true)
		return m, scheduleStatusClear()
	}

	m.overlay = overlayNone
	m.loading = true
	m.hpaScale = hpaScaleState{}

	var cmds []tea.Cmd
	if scaleMinMax {
		m.addLogEntry("DBG", fmt.Sprintf("$ kubectl patch hpa %s --type merge -p '{\"spec\":{\"minReplicas\":%d,\"maxReplicas\":%d}}' -n %s --context %s", m.actionCtx.name, minR, maxR, m.actionCtx.namespace, m.actionCtx.context))
		cmds = append(cmds, m.patchHPAScale(minR, maxR))
	}
	if scaleTarget {
		m.addLogEntry("DBG", fmt.Sprintf("$ kubectl scale %s %s --replicas=%d -n %s --context %s", strings.ToLower(st.targetKind), st.targetName, targetR, m.actionCtx.namespace, m.actionCtx.context))
		cmds = append(cmds, m.scaleHPATarget(st.targetKind, st.targetName, targetR))
	}
	return m, tea.Batch(cmds...)
}

// parseHPAScaleInputs validates and parses only the inputs that are actually
// being applied. When scaleMinMax is set, minReplicas must be >= 1 and
// maxReplicas >= minReplicas; when scaleTarget is set, target replicas >= 0.
// Min/max are not validated for a target-only change (they may be unset when
// prefill from the HPA spec was unavailable).
func parseHPAScaleInputs(st hpaScaleState, scaleMinMax, scaleTarget bool) (minR, maxR, targetR int32, err error) {
	if scaleMinMax {
		minV, errMin := strconv.ParseInt(strings.TrimSpace(st.min.Value), 10, 32)
		maxV, errMax := strconv.ParseInt(strings.TrimSpace(st.max.Value), 10, 32)
		if errMin != nil || errMax != nil || minV < 1 || maxV < 1 {
			return 0, 0, 0, fmt.Errorf("invalid min/max replicas")
		}
		if maxV < minV {
			return 0, 0, 0, fmt.Errorf("max replicas must be >= min replicas")
		}
		minR, maxR = int32(minV), int32(maxV)
	}
	if scaleTarget {
		tv, errT := strconv.ParseInt(strings.TrimSpace(st.target.Value), 10, 32)
		if errT != nil || tv < 0 {
			return 0, 0, 0, fmt.Errorf("invalid target replicas")
		}
		targetR = int32(tv)
	}
	return minR, maxR, targetR, nil
}

// patchHPAScale returns a command that patches the selected HPA's min/max bounds.
func (m Model) patchHPAScale(minR, maxR int32) tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	rt := m.actionCtx.resourceType
	return m.scheduleK8sCall(scheduler.PriorityCritical, scheduler.KindMutation, fmt.Sprintf("Scale HPA %s → %d-%d", name, minR, maxR), bgtaskTarget(ctx, ns), func(_ context.Context) tea.Msg {
		err := m.client.PatchHPAScale(ctx, ns, rt, name, minR, maxR)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Scaled HPA %s to %d-%d replicas", name, minR, maxR)}
	})
}

// scaleHPATarget returns a command that scales the HPA's target workload to the
// given replica count (reusing the generic ScaleResource path).
func (m Model) scaleHPATarget(kind, name string, replicas int32) tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	return m.scheduleK8sCall(scheduler.PriorityCritical, scheduler.KindMutation, fmt.Sprintf("Scale %s/%s → %d", kind, name, replicas), bgtaskTarget(ctx, ns), func(_ context.Context) tea.Msg {
		err := m.client.ScaleResource(ctx, ns, name, kind, replicas)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Scaled %s to %d replicas", name, replicas)}
	})
}
