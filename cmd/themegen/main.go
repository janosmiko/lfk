// Command themegen parses ghostty terminal theme files and generates a Go source
// file containing all themes as built-in colorschemes for lfk.
//
// Usage:
//
//	go run ./cmd/themegen --input-dir=themes/ghostty --output=internal/ui/colorschemes_gen.go
package main

import (
	"bufio"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"
)

func main() {
	inputDir := flag.String("input-dir", "themes/ghostty", "directory containing ghostty theme files")
	output := flag.String("output", "internal/ui/colorschemes_gen.go", "output Go file path")
	skipList := flag.String("skip", "", "comma-separated theme names to skip (hand-crafted)")
	flag.Parse()

	if err := run(*inputDir, *output, *skipList); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(inputDir, output, skipList string) error {
	skip := make(map[string]bool)
	if skipList != "" {
		for _, s := range strings.Split(skipList, ",") {
			skip[strings.TrimSpace(s)] = true
		}
	}

	entries, err := os.ReadDir(inputDir)
	if err != nil {
		return fmt.Errorf("reading input directory %s: %w", inputDir, err)
	}

	var themes []themeEntry
	var skipped, failed int
	seen := make(map[string]bool)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := normalizeName(entry.Name())
		if skip[name] || seen[name] {
			skipped++
			continue
		}
		seen[name] = true

		path := filepath.Join(inputDir, entry.Name())
		td, err := parseThemeFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", entry.Name(), err)
			failed++
			continue
		}

		theme := mapToTheme(td)
		isLight := luminance(mustParseHex(td.Background)) > 0.5

		themes = append(themes, themeEntry{
			Name:    name,
			Theme:   theme,
			IsLight: isLight,
		})
	}

	sort.Slice(themes, func(i, j int) bool {
		return themes[i].Name < themes[j].Name
	})

	return writeOutput(output, themes, skipped, failed)
}

func writeOutput(path string, themes []themeEntry, skipped, failed int) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer func() { _ = f.Close() }()

	data := templateData{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Themes:    themes,
	}

	if err := outputTmpl.Execute(f, data); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	fmt.Fprintf(os.Stderr, "generated %d themes (%d skipped, %d failed) -> %s\n",
		len(themes), skipped, failed, path)
	return nil
}

// --- Theme file parsing ---

type rawTheme struct {
	Background string
	Foreground string
	Palette    [16]string // palette[0..15]
}

type themeData = rawTheme

func parseThemeFile(path string) (rawTheme, error) {
	f, err := os.Open(path)
	if err != nil {
		return rawTheme{}, err
	}
	defer func() { _ = f.Close() }()

	var t rawTheme
	paletteCount := 0

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := parseConfigLine(line)
		if !ok {
			continue
		}

		switch key {
		case "background":
			t.Background = normalizeColor(value)
		case "foreground":
			t.Foreground = normalizeColor(value)
		case "palette":
			// Format: "N=#rrggbb" or "N=rrggbb"
			parts := strings.SplitN(value, "=", 2)
			if len(parts) != 2 {
				continue
			}
			idx := 0
			if _, err := fmt.Sscanf(parts[0], "%d", &idx); err != nil || idx < 0 || idx > 15 {
				continue
			}
			t.Palette[idx] = normalizeColor(parts[1])
			paletteCount++
		}
	}

	if t.Background == "" || t.Foreground == "" {
		return rawTheme{}, fmt.Errorf("missing background or foreground")
	}
	if paletteCount < 8 {
		return rawTheme{}, fmt.Errorf("incomplete palette (%d/16 entries)", paletteCount)
	}

	// Fill missing palette entries with defaults.
	for i := range 16 {
		if t.Palette[i] == "" {
			if i < 8 {
				t.Palette[i] = t.Foreground
			} else {
				t.Palette[i] = t.Palette[i-8] // bright = normal
			}
		}
	}

	return t, nil
}

func parseConfigLine(line string) (key, value string, ok bool) {
	// Ghostty theme format: "key = value"
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), true
}

func normalizeColor(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "#") {
		s = "#" + s
	}
	return strings.ToLower(s)
}

func normalizeName(filename string) string {
	name := strings.TrimSuffix(filename, filepath.Ext(filename))
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.ReplaceAll(name, " ", "-")
	return name
}

// --- Color utilities ---

func parseHex(s string) (r, g, b uint8, err error) {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return 0, 0, 0, fmt.Errorf("invalid hex color: %q", s)
	}
	var ri, gi, bi int
	_, err = fmt.Sscanf(s, "%02x%02x%02x", &ri, &gi, &bi)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid hex color %q: %w", s, err)
	}
	return uint8(ri), uint8(gi), uint8(bi), nil
}

func mustParseHex(s string) (r, g, b uint8) {
	r, g, b, err := parseHex(s)
	if err != nil {
		return 128, 128, 128 // fallback gray
	}
	return r, g, b
}

func toHex(r, g, b uint8) string {
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

// luminance computes relative luminance per sRGB (ITU-R BT.709).
func luminance(r, g, b uint8) float64 {
	linearize := func(v uint8) float64 {
		c := float64(v) / 255.0
		if c <= 0.04045 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*linearize(r) + 0.7152*linearize(g) + 0.0722*linearize(b)
}

func blendColor(c1, c2 string, t float64) string {
	r1, g1, b1 := mustParseHex(c1)
	r2, g2, b2 := mustParseHex(c2)
	lerp := func(a, b uint8) uint8 {
		return uint8(float64(a)*(1-t) + float64(b)*t)
	}
	return toHex(lerp(r1, r2), lerp(g1, g2), lerp(b1, b2))
}

func lighten(hex string, amount float64) string {
	return blendColor(hex, "#ffffff", amount)
}

func darken(hex string, amount float64) string {
	return blendColor(hex, "#000000", amount)
}

// --- Theme mapping ---

func mapToTheme(t rawTheme) themeData {
	isLight := luminance(mustParseHex(t.Background)) > 0.5

	// Derive BarBg: slightly offset from background.
	var barBg, surface string
	if isLight {
		barBg = darken(t.Background, 0.06)
		surface = darken(t.Background, 0.03)
	} else {
		barBg = lighten(t.Background, 0.06)
		surface = lighten(t.Background, 0.03)
	}

	// Use palette 8 (bright black) for border/dimmed if distinct enough,
	// otherwise derive from bg/fg midpoint.
	border := t.Palette[8]
	if border == t.Background || border == t.Foreground {
		border = blendColor(t.Background, t.Foreground, 0.25)
	}

	// Store derived values back into the struct for template output.
	// We reuse rawTheme fields but the template reads from the mapped values below.
	_ = barBg
	_ = surface
	_ = border

	// Return a rawTheme with the mapped colors stored for template use.
	// The template will read Primary, Secondary, etc. from this struct.
	return rawTheme{
		Background: t.Background,
		Foreground: t.Foreground,
		Palette:    t.Palette,
	}
}

// themeFields returns the 13 Theme field values for a rawTheme.
func themeFields(t rawTheme) [13]string {
	isLight := luminance(mustParseHex(t.Background)) > 0.5

	var barBg, surface string
	if isLight {
		barBg = darken(t.Background, 0.06)
		surface = darken(t.Background, 0.03)
	} else {
		barBg = lighten(t.Background, 0.06)
		surface = lighten(t.Background, 0.03)
	}

	border := t.Palette[8]
	if border == t.Background || border == t.Foreground {
		border = blendColor(t.Background, t.Foreground, 0.25)
	}

	return [13]string{
		t.Palette[4], // Primary (blue)
		t.Palette[2], // Secondary (green)
		t.Foreground, // Text
		t.Background, // SelectedFg (bg for contrast)
		t.Palette[4], // SelectedBg (same as primary)
		border,       // Border
		t.Palette[8], // Dimmed (bright black)
		t.Palette[1], // Error (red)
		t.Palette[3], // Warning (yellow)
		t.Palette[5], // Purple (magenta)
		t.Background, // Base
		barBg,        // BarBg
		surface,      // Surface
	}
}

// --- Code generation ---

type themeEntry struct {
	Name    string
	Theme   themeData
	IsLight bool
}

type templateData struct {
	Timestamp string
	Themes    []themeEntry
}

var funcMap = template.FuncMap{
	"fields": themeFields,
}

var outputTmpl = template.Must(template.New("gen").Funcs(funcMap).Parse(`// Code generated by cmd/themegen at {{ .Timestamp }}; DO NOT EDIT.

package ui

// generatedSchemes returns color schemes auto-generated from ghostty terminal themes.
func generatedSchemes() map[string]Theme {
	return map[string]Theme{
{{- range .Themes }}
		{{ printf "%q" .Name }}: {
{{- $f := fields .Theme }}
			Primary:    {{ printf "%q" (index $f 0) }},
			Secondary:  {{ printf "%q" (index $f 1) }},
			Text:       {{ printf "%q" (index $f 2) }},
			SelectedFg: {{ printf "%q" (index $f 3) }},
			SelectedBg: {{ printf "%q" (index $f 4) }},
			Border:     {{ printf "%q" (index $f 5) }},
			Dimmed:     {{ printf "%q" (index $f 6) }},
			Error:      {{ printf "%q" (index $f 7) }},
			Warning:    {{ printf "%q" (index $f 8) }},
			Purple:     {{ printf "%q" (index $f 9) }},
			Base:       {{ printf "%q" (index $f 10) }},
			BarBg:      {{ printf "%q" (index $f 11) }},
			Surface:    {{ printf "%q" (index $f 12) }},
		},
{{- end }}
	}
}

// generatedLightSchemes returns the set of generated schemes classified as light.
func generatedLightSchemes() map[string]bool {
	return map[string]bool{
{{- range .Themes }}
{{- if .IsLight }}
		{{ printf "%q" .Name }}: true,
{{- end }}
{{- end }}
	}
}
`))
