package peco

import "testing"

// TestFuzzyFilter tests a fuzzy filter against various inputs
func TestFuzzyFilter(t *testing.T) {
	testValues := []struct {
		input    string
		query    string
		selected bool
	}{
		{"this is a test to test the fuzzy Filter", "tf", true},  // normal selection
		{"this is a test to test the fuzzy Filter", "wp", false}, // incorrect selection
		{"THIS IS A TEST TO TEST THE FUZZY FILTER", "tu", true},  // case insensitivity
		{"this is a Test to test the fuzzy filter", "Tu", true},  // case sensitivity
		{"this is a Test to test the fUzzy filter", "TU", true},  // case sensitivity
		{"this is a test to test the fuzzy filter", "Tu", false}, // case sensitivity
		{"this is a test to Test the fuzzy filter", "TU", false}, // case sensitivity
		{"日本語は難しいです", "難", true},                                 // kanji
		{"あ、日本語は難しいですよ", "あい", true},                             // hiragana
		{"パソコンは遅いですネ", "ソネ", true},                               // katana
		{"🚴🏻 abcd efgh", "🚴🏻e", true},                            // unicode
	}
	filter := NewFuzzyFilter()
	for i, v := range testValues {
		filter.SetQuery(v.query)
		l := NewRawLine(uint64(i), v.input, false)
		res, err := filter.filter(l)
		if v.selected && err != nil {
			t.Log("Filtering failed.", "input", v.input, "query", v.query, "err", err)
			t.Fail()
		}
		if v.selected && res == nil {
			t.Log("The line should have been selected.", "input", v.input, "query", v.query)
			t.Fail()
		}
		if !v.selected && res != nil {
			t.Log("The line should not have been selected.", "input", v.input, "query", v.query)
			t.Fail()
		}
	}
}
