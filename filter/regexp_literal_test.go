package filter

import (
	"math/rand"
	"reflect"
	"testing"
)

// TestLiteralMatcherEquivalence checks that, whenever newLiteralMatcher accepts a
// term, its ranges are exactly what the compiled regexp reports.
func TestLiteralMatcherEquivalence(t *testing.T) {
	t.Parallel()

	r := rand.New(rand.NewSource(1))
	termAlphabet := []rune("abcABCkKsS019_ xX.é")
	textAlphabet := []rune("abcABCkKsS019_ xX.éÉKſ�")
	randomRunes := func(alphabet []rune, n int) string {
		rs := make([]rune, n)
		for i := range rs {
			rs[i] = alphabet[r.Intn(len(alphabet))]
		}
		return string(rs)
	}

	type mode struct {
		flags     []string
		quotemeta bool
	}
	modes := []mode{{nil, true}, {[]string{"i"}, true}, {nil, false}, {[]string{"i"}, false}}

	accepted := 0
	for i := 0; i < 20000; i++ {
		term := randomRunes(termAlphabet, 1+r.Intn(4))
		text := randomRunes(textAlphabet, r.Intn(40))
		if r.Intn(2) == 0 {
			// Tiny alphabet so terms like "aa" or "aba" can overlap themselves in the text.
			term = randomRunes([]rune("aAb"), 1+r.Intn(3))
			text = randomRunes([]rune("aAb"), r.Intn(30))
		}
		if r.Intn(10) == 0 {
			text += "\xff\xfe" + term
		}
		m := modes[r.Intn(len(modes))]

		rx, err := regexpFor(term, m.flags, m.quotemeta)
		if err != nil {
			continue
		}
		lm := newLiteralMatcher(rx)
		if !lm.ok {
			continue
		}
		accepted++

		var want []int
		for _, loc := range rx.FindAllStringIndex(text, -1) {
			want = append(want, loc[0], loc[1])
		}
		got := lm.appendAll(nil, text)
		if !reflect.DeepEqual(want, got) {
			t.Fatalf("term %q text %q flags %v quotemeta %v: regexp %v, literal %v", term, text, m.flags, m.quotemeta, want, got)
		}
	}
	// The generator must exercise the fast path, or the check proves nothing.
	if accepted < 2000 {
		t.Fatalf("only %d of 20000 cases used the literal fast path", accepted)
	}
}

// TestLiteralMatcherRejectsFoldEdgeLetters pins the letters whose case folding is not ASCII only.
func TestLiteralMatcherRejectsFoldEdgeLetters(t *testing.T) {
	t.Parallel()

	for _, term := range []string{"k", "K", "s", "S", "ask", "this"} {
		rx, err := regexpFor(term, []string{"i"}, true)
		if err != nil {
			t.Fatal(err)
		}
		if newLiteralMatcher(rx).ok {
			t.Errorf("case-insensitive %q must fall back to the regexp", term)
		}
	}
}
