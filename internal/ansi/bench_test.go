package ansi

import "testing"

// BenchmarkParse_MultipleColors benchmarks parsing a line with several ANSI
// color changes, which exercises the parseSGR strings.Split path.
func BenchmarkParse_MultipleColors(b *testing.B) {
	input := "\x1b[1;31mRed Bold\x1b[0m normal \x1b[38;5;196mExtended\x1b[0m \x1b[4;32mGreen UL\x1b[0m"

	b.ReportAllocs()
	for b.Loop() {
		Parse(input)
	}
}

// BenchmarkParse_TrueColor benchmarks truecolor parsing (38;2;R;G;B).
func BenchmarkParse_TrueColor(b *testing.B) {
	input := "\x1b[38;2;255;128;0mOrange\x1b[0m \x1b[48;2;0;128;255mBlueBG\x1b[0m"

	b.ReportAllocs()
	for b.Loop() {
		Parse(input)
	}
}

// BenchmarkParseSGR benchmarks the raw SGR parameter parsing.
func BenchmarkParseSGR(b *testing.B) {
	var fg, bg Attribute

	b.Run("simple", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			fg, bg = ColorDefault, ColorDefault
			parseSGR("1;31;42", &fg, &bg)
		}
	})

	b.Run("256color", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			fg, bg = ColorDefault, ColorDefault
			parseSGR("38;5;196", &fg, &bg)
		}
	})

	b.Run("truecolor", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			fg, bg = ColorDefault, ColorDefault
			parseSGR("38;2;255;128;0", &fg, &bg)
		}
	})
}
