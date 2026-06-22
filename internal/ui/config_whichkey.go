package ui

// applyWhichKey applies the which_key_enabled and which_key_delay_ms settings.
func applyWhichKey(cfg configFile) {
	if cfg.WhichKeyEnabled != nil {
		ConfigWhichKeyEnabled = *cfg.WhichKeyEnabled
	}
	if cfg.WhichKeyDelayMs != nil {
		ConfigWhichKeyDelayMs = max(0, min(*cfg.WhichKeyDelayMs, 2000))
	}
}
