package ansi

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParse_NoANSI(t *testing.T) {
	r := Parse("Hello World")
	require.Equal(t, "Hello World", r.Stripped)
	require.Nil(t, r.Attrs)
}

func TestParse_EmptyString(t *testing.T) {
	r := Parse("")
	require.Equal(t, "", r.Stripped)
	require.Nil(t, r.Attrs)
}

func TestParse_BasicForegroundColors(t *testing.T) {
	r := Parse("\x1b[31mRed\x1b[0m")
	require.Equal(t, "Red", r.Stripped)
	require.Len(t, r.Attrs, 1)
	require.Equal(t, ColorRed, r.Attrs[0].Fg)
	require.Equal(t, ColorDefault, r.Attrs[0].Bg)
	require.Equal(t, 3, r.Attrs[0].Length)
}

func TestParse_BasicBackgroundColors(t *testing.T) {
	r := Parse("\x1b[42mGreen BG\x1b[0m")
	require.Equal(t, "Green BG", r.Stripped)
	require.Len(t, r.Attrs, 1)
	require.Equal(t, ColorDefault, r.Attrs[0].Fg)
	require.Equal(t, ColorGreen, r.Attrs[0].Bg)
	require.Equal(t, 8, r.Attrs[0].Length)
}

func TestParse_BoldAttribute(t *testing.T) {
	r := Parse("\x1b[1;31mBold Red\x1b[0m")
	require.Equal(t, "Bold Red", r.Stripped)
	require.Len(t, r.Attrs, 1)
	require.Equal(t, ColorRed|AttrBold, r.Attrs[0].Fg)
	require.Equal(t, 8, r.Attrs[0].Length)
}

func TestParse_UnderlineAttribute(t *testing.T) {
	r := Parse("\x1b[4mUnderline\x1b[0m")
	require.Equal(t, "Underline", r.Stripped)
	require.Len(t, r.Attrs, 1)
	require.Equal(t, AttrUnderline, r.Attrs[0].Fg)
}

func TestParse_ReverseAttribute(t *testing.T) {
	r := Parse("\x1b[7mReverse\x1b[0m")
	require.Equal(t, "Reverse", r.Stripped)
	require.Len(t, r.Attrs, 1)
	require.Equal(t, AttrReverse, r.Attrs[0].Fg)
}

func TestParse_MultipleSegments(t *testing.T) {
	r := Parse("AAA\x1b[31mBBB\x1b[0mCCC")
	require.Equal(t, "AAABBBCCC", r.Stripped)
	require.Len(t, r.Attrs, 3)

	require.Equal(t, ColorDefault, r.Attrs[0].Fg)
	require.Equal(t, 3, r.Attrs[0].Length)

	require.Equal(t, ColorRed, r.Attrs[1].Fg)
	require.Equal(t, 3, r.Attrs[1].Length)

	require.Equal(t, ColorDefault, r.Attrs[2].Fg)
	require.Equal(t, 3, r.Attrs[2].Length)
}

func TestParse_256Color(t *testing.T) {
	// 38;5;196 = 256-color fg (bright red, index 196)
	r := Parse("\x1b[38;5;196mColor256\x1b[0m")
	require.Equal(t, "Color256", r.Stripped)
	require.Len(t, r.Attrs, 1)
	require.Equal(t, Attribute(197), r.Attrs[0].Fg) // 196+1 for palette encoding
}

func TestParse_256ColorBackground(t *testing.T) {
	r := Parse("\x1b[48;5;21mBlueBG\x1b[0m")
	require.Equal(t, "BlueBG", r.Stripped)
	require.Len(t, r.Attrs, 1)
	require.Equal(t, Attribute(22), r.Attrs[0].Bg) // 21+1
}

func TestParse_TrueColor(t *testing.T) {
	// 38;2;255;128;0 = truecolor fg (orange)
	r := Parse("\x1b[38;2;255;128;0mOrange\x1b[0m")
	require.Equal(t, "Orange", r.Stripped)
	require.Len(t, r.Attrs, 1)
	expected := Attribute((255<<16)|(128<<8)) | AttrTrueColor
	require.Equal(t, expected, r.Attrs[0].Fg)
}

func TestParse_TrueColorBackground(t *testing.T) {
	r := Parse("\x1b[48;2;0;128;255mBG\x1b[0m")
	require.Equal(t, "BG", r.Stripped)
	require.Len(t, r.Attrs, 1)
	expected := Attribute((0<<16)|(128<<8)|255) | AttrTrueColor
	require.Equal(t, expected, r.Attrs[0].Bg)
}

func TestParse_Reset(t *testing.T) {
	r := Parse("\x1b[1;31mBold Red\x1b[0m Normal")
	require.Equal(t, "Bold Red Normal", r.Stripped)
	require.Len(t, r.Attrs, 2)
	require.Equal(t, ColorRed|AttrBold, r.Attrs[0].Fg)
	require.Equal(t, 8, r.Attrs[0].Length)
	require.Equal(t, ColorDefault, r.Attrs[1].Fg)
	require.Equal(t, 7, r.Attrs[1].Length)
}

func TestParse_MalformedIncomplete(t *testing.T) {
	// Incomplete sequence at end
	r := Parse("Hello\x1b[")
	require.Equal(t, "Hello", r.Stripped)
}

func TestParse_MalformedNoTerminator(t *testing.T) {
	// ESC[31W — 'W' is a valid CSI terminator (non-SGR), so it is stripped
	// and "orld" remains as regular text
	r := Parse("Hello\x1b[31World")
	require.Equal(t, "Helloorld", r.Stripped)
}

func TestParse_NonSGRSequence(t *testing.T) {
	// ESC[2J is "clear screen" (not SGR) — should be stripped, not rendered
	r := Parse("\x1b[2JHello")
	require.Equal(t, "Hello", r.Stripped)
}

func TestParse_EmptyReset(t *testing.T) {
	// ESC[m is equivalent to ESC[0m
	r := Parse("\x1b[31mRed\x1b[mNormal")
	require.Equal(t, "RedNormal", r.Stripped)
	require.Len(t, r.Attrs, 2)
	require.Equal(t, ColorRed, r.Attrs[0].Fg)
	require.Equal(t, ColorDefault, r.Attrs[1].Fg)
}

func TestParse_DefaultFgBgReset(t *testing.T) {
	r := Parse("\x1b[31;42mColored\x1b[39;49mReset")
	require.Equal(t, "ColoredReset", r.Stripped)
	require.Len(t, r.Attrs, 2)
	require.Equal(t, ColorRed, r.Attrs[0].Fg)
	require.Equal(t, ColorGreen, r.Attrs[0].Bg)
	// code 39 resets fg, code 49 resets bg
	require.Equal(t, ColorDefault, r.Attrs[1].Fg)
	require.Equal(t, ColorDefault, r.Attrs[1].Bg)
}

func TestParse_ComplexCombined(t *testing.T) {
	// Bold underline red on blue background
	r := Parse("\x1b[1;4;31;44mStyled\x1b[0m")
	require.Equal(t, "Styled", r.Stripped)
	require.Len(t, r.Attrs, 1)
	require.Equal(t, ColorRed|AttrBold|AttrUnderline, r.Attrs[0].Fg)
	require.Equal(t, ColorBlue, r.Attrs[0].Bg)
}

func TestExtractSegment_Nil(t *testing.T) {
	require.Nil(t, ExtractSegment(nil, 0, 5))
}

func TestExtractSegment_EmptyRange(t *testing.T) {
	attrs := []AttrSpan{{Fg: ColorRed, Bg: ColorDefault, Length: 10}}
	require.Nil(t, ExtractSegment(attrs, 5, 5))
}

func TestExtractSegment_FullSpan(t *testing.T) {
	attrs := []AttrSpan{
		{Fg: ColorRed, Bg: ColorDefault, Length: 3},
		{Fg: ColorDefault, Bg: ColorDefault, Length: 7},
		{Fg: ColorBlue, Bg: ColorDefault, Length: 5},
	}
	result := ExtractSegment(attrs, 0, 15)
	require.Equal(t, attrs, result)
}

func TestExtractSegment_MiddleSlice(t *testing.T) {
	attrs := []AttrSpan{
		{Fg: ColorRed, Bg: ColorDefault, Length: 5},
		{Fg: ColorBlue, Bg: ColorDefault, Length: 5},
		{Fg: ColorGreen, Bg: ColorDefault, Length: 5},
	}
	// Extract runes 3..8 — partial Red(2) + full Blue(5) + partial Green(1)
	result := ExtractSegment(attrs, 3, 12)
	require.Len(t, result, 3)
	require.Equal(t, 2, result[0].Length) // Red: runes 3..5
	require.Equal(t, ColorRed, result[0].Fg)
	require.Equal(t, 5, result[1].Length) // Blue: runes 5..10
	require.Equal(t, ColorBlue, result[1].Fg)
	require.Equal(t, 2, result[2].Length) // Green: runes 10..12
	require.Equal(t, ColorGreen, result[2].Fg)
}

func TestExtractSegment_SingleSpanSlice(t *testing.T) {
	attrs := []AttrSpan{
		{Fg: ColorRed, Bg: ColorDefault, Length: 10},
	}
	result := ExtractSegment(attrs, 2, 7)
	require.Len(t, result, 1)
	require.Equal(t, 5, result[0].Length)
	require.Equal(t, ColorRed, result[0].Fg)
}
