package app

import (
	"strconv"
	"time"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

// capturesPseudoItems builds the __captures__ pseudo-resource row list from
// the CaptureManager state. Mirrors portForwardItems for consistency.
//
// The row Kind is "__captures__" and the capture ID is stored both in Extra
// (as a plain string) and in Columns under the key "ID" so that action
// handlers can resolve back to the manager entry via getCaptureIDFromItem or
// getCaptureIDFromColumns.
func capturesPseudoItems(mgr *k8s.CaptureManager) []model.Item {
	if mgr == nil {
		return nil
	}
	entries := mgr.Entries()
	out := make([]model.Item, 0, len(entries))
	for _, e := range entries {
		idStr := strconv.Itoa(e.ID)
		out = append(out, model.Item{
			Name:      e.Request.PodName,
			Kind:      "__captures__",
			Namespace: e.Request.Namespace,
			Status:    string(e.Status),
			Extra:     idStr,
			Columns: []model.KeyValue{
				{Key: "ID", Value: idStr},
				{Key: "STATUS", Value: string(e.Status)},
				{Key: "BACKEND", Value: string(e.Request.Backend)},
				{Key: "PACKETS", Value: strconv.FormatInt(e.PacketCount, 10)},
				{Key: "BYTES", Value: strconv.FormatInt(e.ByteCount, 10)},
				{Key: "ELAPSED", Value: time.Since(e.StartedAt).Truncate(time.Second).String()},
				{Key: "FILE", Value: e.OutputPath},
			},
		})
	}
	return out
}
