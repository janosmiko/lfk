package ui

import (
	"bufio"
	_ "embed"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

//go:embed colorschemes.tsv
var colorschemesTSV string

const colorschemesColumns = 15

func parseColorschemes() (map[string]Theme, map[string]bool) {
	return parseColorschemesTSV(colorschemesTSV)
}

// parseColorschemesTSV parses TSV rows of: name, isLight, then the 13 Theme
// fields in struct order. A malformed row is a build-time programmer error.
func parseColorschemesTSV(data string) (map[string]Theme, map[string]bool) {
	schemes := make(map[string]Theme)
	light := make(map[string]bool)

	scanner := bufio.NewScanner(strings.NewReader(data))
	row := 0
	for scanner.Scan() {
		row++
		line := scanner.Text()
		if line == "" {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) != colorschemesColumns {
			panic(fmt.Sprintf("colorschemes.tsv row %d: expected %d columns, got %d", row, colorschemesColumns, len(cols)))
		}

		name := cols[0]
		isLight, err := strconv.ParseBool(cols[1])
		if err != nil {
			panic(fmt.Sprintf("colorschemes.tsv row %d: invalid isLight %q: %v", row, cols[1], err))
		}

		schemes[name] = Theme{
			Primary:    cols[2],
			Secondary:  cols[3],
			Text:       cols[4],
			SelectedFg: cols[5],
			SelectedBg: cols[6],
			Border:     cols[7],
			Dimmed:     cols[8],
			Error:      cols[9],
			Warning:    cols[10],
			Purple:     cols[11],
			Base:       cols[12],
			BarBg:      cols[13],
			Surface:    cols[14],
		}
		if isLight {
			light[name] = true
		}
	}
	if err := scanner.Err(); err != nil {
		panic(fmt.Sprintf("colorschemes.tsv: %v", err))
	}

	return schemes, light
}

var loadColorschemes = sync.OnceValues(parseColorschemes)

// generatedSchemes returns color schemes auto-generated from ghostty terminal themes.
func generatedSchemes() map[string]Theme {
	schemes, _ := loadColorschemes()
	return schemes
}

// generatedLightSchemes returns the set of generated schemes classified as light.
func generatedLightSchemes() map[string]bool {
	_, light := loadColorschemes()
	return light
}
