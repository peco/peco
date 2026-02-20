package keyseq

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func checkNode(t *testing.T, node Node, size int, data nodeData) {
	require.NotNil(t, node, "Nil node:", data)
	require.Equal(t, size, node.Size(), "Unexpected childrens")
	d := node.Value().(*nodeData)
	require.NotNil(t, d, "Nil data:", data, node)
	if data.pattern != nil {
		require.True(t, d.pattern.Equals(*data.pattern), "Pattern unmatched:", data, node, *d.pattern)
	}
	if data.value != nil {
		require.Equal(t, data.value, d.value, "Value unmatched:", data, node, d.value)
	}
	require.NotNil(t, d.failure, "Nil failure:", data, node)
	require.Equal(t, data.failure, d.failure, "Failure unmatched: data=%+v node=%+v d.failure=%+v",
		data, node, d.failure)
}

func invalidData(failure Node) nodeData {
	return nodeData{
		failure: failure.(*TernaryNode),
	}
}

func validData(pattern KeyList, value any, failure Node) nodeData {
	return nodeData{
		pattern: &pattern,
		value:   value,
		failure: failure.(*TernaryNode),
	}
}

func newTestMatcher() *Matcher {
	m := NewMatcher()
	m.Add(KeyList{Key{0, KeyCtrlA, rune(0)}, Key{0, KeyCtrlB, rune(0)}}, 2)
	m.Add(KeyList{Key{0, KeyCtrlB, rune(0)}, Key{0, KeyCtrlC, rune(0)}}, 4)
	m.Add(KeyList{Key{0, KeyCtrlB, rune(0)}, Key{0, KeyCtrlA, rune(0)}, Key{0, KeyCtrlB, rune(0)}}, 6)
	m.Add(KeyList{Key{0, KeyCtrlD, rune(0)}}, 7)
	m.Add(KeyList{Key{0, KeyCtrlA, rune(0)}, Key{0, KeyCtrlB, rune(0)}, Key{0, KeyCtrlC, rune(0)}, Key{0, KeyCtrlD, rune(0)}, Key{0, KeyCtrlE, rune(0)}}, 10)
	m.Compile()
	return m
}

func TestTree(t *testing.T) {
	m := newTestMatcher()
	// Check tree structure.
	r := m.Root()
	checkNode(t, r, 3, invalidData(r))
	n1 := r.Get(NewKeyFromKey(KeyCtrlA))
	checkNode(t, n1, 1, invalidData(r))
	n3 := r.Get(NewKeyFromKey(KeyCtrlB))
	checkNode(t, n3, 2, invalidData(r))
	n7 := r.Get(NewKeyFromKey(KeyCtrlD))
	checkNode(t, n7, 0, invalidData(r))
	n2 := n1.Get(NewKeyFromKey(KeyCtrlB))
	checkNode(t, n2, 1, validData(KeyList{NewKeyFromKey(KeyCtrlA), NewKeyFromKey(KeyCtrlB)}, 2, n3))
	n4 := n3.Get(NewKeyFromKey(KeyCtrlC))
	checkNode(t, n4, 0, validData(KeyList{NewKeyFromKey(KeyCtrlB), NewKeyFromKey(KeyCtrlC)}, 4, r))
	n5 := n3.Get(NewKeyFromKey(KeyCtrlA))
	checkNode(t, n5, 1, invalidData(n1))
	n8 := n2.Get(NewKeyFromKey(KeyCtrlC))
	checkNode(t, n8, 1, invalidData(n4))
	n6 := n5.Get(NewKeyFromKey(KeyCtrlB))
	checkNode(t, n6, 0, validData(KeyList{NewKeyFromKey(KeyCtrlB), NewKeyFromKey(KeyCtrlA), NewKeyFromKey(KeyCtrlB)}, 6, n2))
	n9 := n8.Get(NewKeyFromKey(KeyCtrlD))
	checkNode(t, n9, 1, invalidData(n7))
	n10 := n9.Get(NewKeyFromKey(KeyCtrlE))
	checkNode(t, n10, 0, validData(KeyList{NewKeyFromKey(KeyCtrlA), NewKeyFromKey(KeyCtrlB), NewKeyFromKey(KeyCtrlC), NewKeyFromKey(KeyCtrlD), NewKeyFromKey(KeyCtrlE)}, 10, r))
}
