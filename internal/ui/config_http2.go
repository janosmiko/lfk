// Package ui — config_http2.go
// The disable_http2 setting and its apply hook.
package ui

// ConfigDisableHTTP2 escapes a proxy that cannot carry a watch: on HTTP/2
// every list and watch shares one connection, so resetting the watch takes
// the lists with it (#694). main.go relays it as the DISABLE_HTTP2 env var.
var ConfigDisableHTTP2 bool

// applyDisableHTTP2 wires the config field into its runtime global.
func applyDisableHTTP2(cfg configFile) {
	if cfg.DisableHTTP2 != nil {
		ConfigDisableHTTP2 = *cfg.DisableHTTP2
	}
}
