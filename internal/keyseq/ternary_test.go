package keyseq

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBalance(t *testing.T) {
	trie := NewTernaryTrie()

	list := make([]Key, 0, 15)
	for i := range 15 {
		list = append(list, Key{0, 0, rune(i)})
	}

	for i, k := range list {
		trie.Put(KeyList{k}, i)
	}
	require.Equal(t, 15, trie.Size())
	trie.Balance()

	// After balancing, all keys must still be retrievable with correct values.
	for i, k := range list {
		node := trie.Get(k)
		require.NotNil(t, node, "key %d should be found after Balance", i)
		require.Equal(t, i, node.Value(), "value for key %d should be %d", i, i)
	}

	// Size must be unchanged after balancing.
	require.Equal(t, 15, trie.Size())
}

func TestBalancePreservesMultiKeySequences(t *testing.T) {
	trie := NewTernaryTrie()

	// Insert multi-key sequences
	k1 := KeyList{{0, 0, 'a'}, {0, 0, 'b'}}
	k2 := KeyList{{0, 0, 'a'}, {0, 0, 'c'}}
	k3 := KeyList{{0, 0, 'x'}}

	trie.Put(k1, "ab")
	trie.Put(k2, "ac")
	trie.Put(k3, "x")

	trie.Balance()

	require.Equal(t, "ab", trie.GetList(k1).Value())
	require.Equal(t, "ac", trie.GetList(k2).Value())
	require.Equal(t, "x", trie.GetList(k3).Value())
}
