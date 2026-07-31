// Package ui - config_delete_policy.go
// The delete_propagation_policy setting: which cascade policy the delete
// confirm dialog starts on.
package ui

import (
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
)

// ConfigDeletePropagationPolicy is the cascade policy a delete confirm opens
// with. Set via the `delete_propagation_policy:` config key.
//
// Background matches kubectl's default. It is deliberately not left unset:
// sending no policy defers to the API server, which still defaults batch/v1
// Job deletes to OrphanDependents and would leave their pods running.
var ConfigDeletePropagationPolicy = model.DeletePropagationBackground

// applyDeletePropagationPolicy validates and applies the
// delete_propagation_policy config value. Empty keeps the compiled default;
// unknown values warn and keep it too.
func applyDeletePropagationPolicy(raw string) {
	if raw == "" {
		return
	}
	policy, ok := model.ParseDeletePropagation(raw)
	if !ok {
		logger.Warn("Invalid delete_propagation_policy; using default",
			"accepted", []string{
				string(model.DeletePropagationBackground),
				string(model.DeletePropagationForeground),
				string(model.DeletePropagationOrphan),
				string(model.DeletePropagationNone),
			},
			"default", string(model.DeletePropagationBackground))
		return
	}
	ConfigDeletePropagationPolicy = policy
}
