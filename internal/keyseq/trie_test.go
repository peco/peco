package keyseq

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func checkTrieNode(t *testing.T, n Node, k Key, value int) {
	require.NotNil(t, n, "TrieNode is null")
	require.Equal(t, k, n.Label(), "TrieNode.Label()")
	require.Equal(t, value, n.Value().(int), "TrieNode.Value()")
}

func TestTrie(t *testing.T) {
	trie := NewTrie()
	for i := 1; i <= 5; i++ {
		trie.Put(KeyList{Key{0, 0, rune(i)}}, 111*i)
	}

	nodes := Children(trie.Root())
	for i := range 5 {
		checkTrieNode(t, nodes[i], Key{0, 0, rune(i + 1)}, 111*(i+1))
	}

	require.Equal(t, 5, trie.Size())
}

func TestNotFound(t *testing.T) {
	trie := NewTrie()
	require.Nil(t, trie.Get(Key{999, 999, 'a'}))
}
