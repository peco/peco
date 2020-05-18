package keyseq

import (
	"testing"

	"github.com/nsf/termbox-go"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func checkNode(t *testing.T, node Node, size int, data nodeData) {
	if node == nil {
		t.Error("Nil node:", data)
	}
	if node.Size() != size {
		t.Errorf("Unexpected childrens: %d != %d", node.Size(), size)
	}
	d := node.Value().(*nodeData)
	if d == nil {
		t.Error("Nil data:", data, node)
	}
	if data.pattern != nil && !d.pattern.Equals(*data.pattern) {
		t.Error("Pattern unmatched:", data, node, *d.pattern)
	}
	if data.value != nil && d.value != data.value {
		t.Error("Value unmatched:", data, node, d.value)
	}
	if d.failure == nil {
		t.Error("Nil failure:", data, node)
	} else if d.failure != data.failure {
		t.Errorf("Failure unmatched: data=%+v node=%+v d.failure=%+v",
			data, node, d.failure)
	}
}

func invalidData(failure Node) nodeData {
	return nodeData{
		failure: failure.(*TernaryNode),
	}
}

func validData(pattern KeyList, value interface{}, failure Node) nodeData {
	return nodeData{
		pattern: &pattern,
		value:   value,
		failure: failure.(*TernaryNode),
	}
}

func newTestMatcher() (*Matcher, error) {
	m := NewMatcher()
	m.Add(KeyList{Key{0, termbox.KeyCtrlA, rune(0)}, Key{0, termbox.KeyCtrlB, rune(0)}}, 2)
	m.Add(KeyList{Key{0, termbox.KeyCtrlB, rune(0)}, Key{0, termbox.KeyCtrlC, rune(0)}}, 4)
	m.Add(KeyList{Key{0, termbox.KeyCtrlB, rune(0)}, Key{0, termbox.KeyCtrlA, rune(0)}, Key{0, termbox.KeyCtrlB, rune(0)}}, 6)
	m.Add(KeyList{Key{0, termbox.KeyCtrlD, rune(0)}}, 7)
	m.Add(KeyList{Key{0, termbox.KeyCtrlA, rune(0)}, Key{0, termbox.KeyCtrlB, rune(0)}, Key{0, termbox.KeyCtrlC, rune(0)}, Key{0, termbox.KeyCtrlD, rune(0)}, Key{0, termbox.KeyCtrlE, rune(0)}}, 10)
	if err := m.Compile(); err != nil {
		return nil, errors.Wrap(err, `failed to compile`)
	}
	return m, nil
}

func TestTree(t *testing.T) {
	m, err := newTestMatcher()
	if !assert.NoError(t, err, `creating new matcher should succeed`) {
		return
	}

	// Check tree structure.
	r := m.Root()
	checkNode(t, r, 3, invalidData(r))
	n1 := r.Get(NewKeyFromKey(termbox.KeyCtrlA))
	checkNode(t, n1, 1, invalidData(r))
	n3 := r.Get(NewKeyFromKey(termbox.KeyCtrlB))
	checkNode(t, n3, 2, invalidData(r))
	n7 := r.Get(NewKeyFromKey(termbox.KeyCtrlD))
	checkNode(t, n7, 0, invalidData(r))
	n2 := n1.Get(NewKeyFromKey(termbox.KeyCtrlB))
	checkNode(t, n2, 1, validData(KeyList{NewKeyFromKey(termbox.KeyCtrlA), NewKeyFromKey(termbox.KeyCtrlB)}, 2, n3))
	n4 := n3.Get(NewKeyFromKey(termbox.KeyCtrlC))
	checkNode(t, n4, 0, validData(KeyList{NewKeyFromKey(termbox.KeyCtrlB), NewKeyFromKey(termbox.KeyCtrlC)}, 4, r))
	n5 := n3.Get(NewKeyFromKey(termbox.KeyCtrlA))
	checkNode(t, n5, 1, invalidData(n1))
	n8 := n2.Get(NewKeyFromKey(termbox.KeyCtrlC))
	checkNode(t, n8, 1, invalidData(n4))
	n6 := n5.Get(NewKeyFromKey(termbox.KeyCtrlB))
	checkNode(t, n6, 0, validData(KeyList{NewKeyFromKey(termbox.KeyCtrlB), NewKeyFromKey(termbox.KeyCtrlA), NewKeyFromKey(termbox.KeyCtrlB)}, 6, n2))
	n9 := n8.Get(NewKeyFromKey(termbox.KeyCtrlD))
	checkNode(t, n9, 1, invalidData(n7))
	n10 := n9.Get(NewKeyFromKey(termbox.KeyCtrlE))
	checkNode(t, n10, 0, validData(KeyList{NewKeyFromKey(termbox.KeyCtrlA), NewKeyFromKey(termbox.KeyCtrlB), NewKeyFromKey(termbox.KeyCtrlC), NewKeyFromKey(termbox.KeyCtrlD), NewKeyFromKey(termbox.KeyCtrlE)}, 10, r))
}
