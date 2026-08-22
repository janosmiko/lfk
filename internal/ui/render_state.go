package ui

import "sync"

// RenderMu serialises a whole render pass: a frame needs every package-level
// Active* and Nyan* variable to come from the same Model. Bubble Tea renders on
// one goroutine, so only tests contend, and without this none can use t.Parallel().
var RenderMu sync.Mutex
