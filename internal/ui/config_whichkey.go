package ui

// applyWhichKey applies the which_key_enabled, which_key_delay_ms,
// which_key_leader_delay_ms and which_key_grouped settings.
func applyWhichKey(cfg configFile) {
	if cfg.WhichKeyGrouped != nil {
		ConfigWhichKeyGrouped = *cfg.WhichKeyGrouped
	}
	if cfg.WhichKeyEnabled != nil {
		ConfigWhichKeyEnabled = *cfg.WhichKeyEnabled
	}
	if cfg.WhichKeyDelayMs != nil {
		ConfigWhichKeyDelayMs = max(0, min(*cfg.WhichKeyDelayMs, 2000))
	}
	if cfg.WhichKeyLeaderDelayMs != nil {
		ConfigWhichKeyLeaderDelayMs = max(0, min(*cfg.WhichKeyLeaderDelayMs, 2000))
	}
}
