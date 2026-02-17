// Package ansi provides ANSI SGR escape sequence parsing for peco.
// It extracts color/style attributes from input text and produces
// run-length encoded attribute spans alongside stripped plain text.
package ansi

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// Attribute mirrors peco.Attribute so we avoid an import cycle.
// The values are identical and can be cast directly.
type Attribute = uint32

// Named palette color constants (matching peco.Color* values).
const (
	ColorDefault Attribute = 0x0000
	ColorBlack   Attribute = 0x0001
	ColorRed     Attribute = 0x0002
	ColorGreen   Attribute = 0x0003
	ColorYellow  Attribute = 0x0004
	ColorBlue    Attribute = 0x0005
	ColorMagenta Attribute = 0x0006
	ColorCyan    Attribute = 0x0007
	ColorWhite   Attribute = 0x0008
)

const (
	AttrTrueColor Attribute = 0x01000000
	AttrBold      Attribute = 0x02000000
	AttrUnderline Attribute = 0x04000000
	AttrReverse   Attribute = 0x08000000
)

// basicFgColors maps SGR codes 30-37 to palette colors.
var basicFgColors = [8]Attribute{
	ColorBlack, ColorRed, ColorGreen, ColorYellow,
	ColorBlue, ColorMagenta, ColorCyan, ColorWhite,
}

// AttrSpan represents a run of characters sharing identical ANSI attributes.
type AttrSpan struct {
	Fg     Attribute
	Bg     Attribute
	Length int // number of runes
}

// ParseResult contains the output of ANSI parsing.
type ParseResult struct {
	Stripped string     // text with ANSI codes removed
	Attrs    []AttrSpan // run-length encoded attributes; nil if no ANSI codes found
}

// Parse parses ANSI SGR sequences from input and returns the stripped text
// along with run-length encoded per-character attributes.
// If no ANSI escape sequences are found, Attrs is nil.
func Parse(input string) ParseResult {
	// Fast path: if no ESC character, return as-is
	if !strings.ContainsRune(input, '\x1b') {
		return ParseResult{Stripped: input, Attrs: nil}
	}

	var (
		out   strings.Builder
		spans []AttrSpan
		curFg = ColorDefault
		curBg = ColorDefault
		count int // runes in current span
	)

	out.Grow(len(input))

	flush := func() {
		if count > 0 {
			spans = append(spans, AttrSpan{Fg: curFg, Bg: curBg, Length: count})
			count = 0
		}
	}

	i := 0
	for i < len(input) {
		if input[i] == '\x1b' && i+1 < len(input) && input[i+1] == '[' {
			// Found CSI sequence: ESC [
			j := i + 2
			// Scan for the terminating byte (0x40-0x7E)
			for j < len(input) && input[j] >= 0x20 && input[j] <= 0x3F {
				j++
			}
			if j >= len(input) {
				// Incomplete sequence at end of string: skip it
				i = j
				continue
			}
			terminator := input[j]
			if terminator == 'm' {
				// SGR sequence
				params := input[i+2 : j]
				flush()
				parseSGR(params, &curFg, &curBg)
			}
			// Skip the entire sequence (including non-SGR ones)
			i = j + 1
			continue
		}

		// Regular character
		r, size := utf8.DecodeRuneInString(input[i:])
		if r == utf8.RuneError && size == 1 {
			r = '?'
		}
		out.WriteRune(r)
		count++
		i += size
	}

	flush()

	return ParseResult{
		Stripped: out.String(),
		Attrs:    spans,
	}
}

// parseSGR interprets SGR parameters (the part between ESC[ and m).
// It modifies fg and bg in place based on the parameter codes.
func parseSGR(params string, fg, bg *Attribute) {
	if params == "" || params == "0" {
		// Reset all
		*fg = ColorDefault
		*bg = ColorDefault
		return
	}

	parts := strings.Split(params, ";")
	for i := 0; i < len(parts); i++ {
		code, err := strconv.Atoi(parts[i])
		if err != nil {
			continue
		}

		switch {
		case code == 0:
			// Reset
			*fg = ColorDefault
			*bg = ColorDefault

		case code == 1:
			*fg |= AttrBold
		case code == 4:
			*fg |= AttrUnderline
		case code == 7:
			*fg |= AttrReverse

		// Basic foreground colors 30-37
		case code >= 30 && code <= 37:
			// Preserve attribute flags, set new color
			flags := *fg & (AttrBold | AttrUnderline | AttrReverse)
			*fg = basicFgColors[code-30] | flags

		// Basic background colors 40-47
		case code >= 40 && code <= 47:
			*bg = basicFgColors[code-40]

		// 256-color or truecolor foreground: 38;5;N or 38;2;R;G;B
		case code == 38:
			if i+1 < len(parts) {
				mode, _ := strconv.Atoi(parts[i+1])
				switch mode {
				case 5: // 256-color: 38;5;N
					if i+2 < len(parts) {
						n, _ := strconv.Atoi(parts[i+2])
						if n >= 0 && n <= 255 {
							flags := *fg & (AttrBold | AttrUnderline | AttrReverse)
							*fg = Attribute(n+1) | flags
						}
						i += 2
					}
				case 2: // Truecolor: 38;2;R;G;B
					if i+4 < len(parts) {
						r, _ := strconv.Atoi(parts[i+2])
						g, _ := strconv.Atoi(parts[i+3])
						b, _ := strconv.Atoi(parts[i+4])
						flags := *fg & (AttrBold | AttrUnderline | AttrReverse)
						*fg = Attribute((r<<16)|(g<<8)|b) | AttrTrueColor | flags
						i += 4
					}
				default:
					i++
				}
			}

		// 256-color or truecolor background: 48;5;N or 48;2;R;G;B
		case code == 48:
			if i+1 < len(parts) {
				mode, _ := strconv.Atoi(parts[i+1])
				switch mode {
				case 5: // 256-color: 48;5;N
					if i+2 < len(parts) {
						n, _ := strconv.Atoi(parts[i+2])
						if n >= 0 && n <= 255 {
							*bg = Attribute(n + 1)
						}
						i += 2
					}
				case 2: // Truecolor: 48;2;R;G;B
					if i+4 < len(parts) {
						r, _ := strconv.Atoi(parts[i+2])
						g, _ := strconv.Atoi(parts[i+3])
						b, _ := strconv.Atoi(parts[i+4])
						*bg = Attribute((r<<16)|(g<<8)|b) | AttrTrueColor
						i += 4
					}
				default:
					i++
				}
			}

		// Default foreground/background reset
		case code == 39:
			flags := *fg & (AttrBold | AttrUnderline | AttrReverse)
			*fg = ColorDefault | flags
		case code == 49:
			*bg = ColorDefault
		}
	}
}

// ExtractSegment extracts ANSI attributes for a rune-range [start, end)
// from the given run-length encoded spans.
// Returns nil if attrs is nil or the segment is empty.
func ExtractSegment(attrs []AttrSpan, start, end int) []AttrSpan {
	if attrs == nil || start >= end {
		return nil
	}

	var result []AttrSpan
	pos := 0

	for _, span := range attrs {
		spanEnd := pos + span.Length

		if spanEnd <= start {
			pos = spanEnd
			continue
		}
		if pos >= end {
			break
		}

		overlapStart := max(start, pos)
		overlapEnd := min(end, spanEnd)
		overlapLen := overlapEnd - overlapStart

		if overlapLen > 0 {
			result = append(result, AttrSpan{
				Fg:     span.Fg,
				Bg:     span.Bg,
				Length: overlapLen,
			})
		}

		pos = spanEnd
	}

	return result
}
