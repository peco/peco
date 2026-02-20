package config

import (
	"fmt"
	"strconv"
	"strings"
)

// HeightSpec represents a height specification that can be either
// an absolute number of lines or a percentage of terminal height.
type HeightSpec struct {
	Value     int
	IsPercent bool
}

// ParseHeightSpec parses a height specification string.
// Valid formats: "10" (absolute lines), "50%" (percentage).
func ParseHeightSpec(s string) (HeightSpec, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return HeightSpec{}, fmt.Errorf("empty height specification")
	}

	if strings.HasSuffix(s, "%") {
		numStr := s[:len(s)-1]
		if strings.Contains(numStr, "%") {
			return HeightSpec{}, fmt.Errorf("invalid height specification: %q", s)
		}
		v, err := strconv.Atoi(numStr)
		if err != nil {
			return HeightSpec{}, fmt.Errorf("invalid height specification: %q", s)
		}
		if v <= 0 {
			return HeightSpec{}, fmt.Errorf("height percentage must be positive: %q", s)
		}
		if v > 100 {
			return HeightSpec{}, fmt.Errorf("height percentage must not exceed 100: %q", s)
		}
		return HeightSpec{Value: v, IsPercent: true}, nil
	}

	v, err := strconv.Atoi(s)
	if err != nil {
		return HeightSpec{}, fmt.Errorf("invalid height specification: %q", s)
	}
	if v <= 0 {
		return HeightSpec{}, fmt.Errorf("height must be positive: %q", s)
	}
	return HeightSpec{Value: v, IsPercent: false}, nil
}

// ChromLines is the number of lines used by the prompt and status bar.
const ChromLines = 2

// Resolve converts the HeightSpec to an absolute number of screen rows,
// clamped to [ChromLines+1, termHeight].
//
// For absolute values, Value is the number of result lines — the prompt
// and status bar are added automatically (total = Value + 2).
// For percentages, Value is the percentage of the full terminal height
// (prompt and status bar are included in that total).
func (h HeightSpec) Resolve(termHeight int) int {
	var height int
	if h.IsPercent {
		height = termHeight * h.Value / 100
	} else {
		height = h.Value + ChromLines
	}

	minHeight := ChromLines + 1 // at least 1 result line
	if height < minHeight {
		height = minHeight
	}
	if height > termHeight {
		height = termHeight
	}
	return height
}
