package keyseq

import (
	"testing"
)

func checkTrieNode(t *testing.T, n Node, k Key, value int) {
	if n == nil {
		t.Fatal("TrieNode is null")
	}
	if l := n.Label(); l != k {
		t.Errorf("TrieNode.Label() expected:'%c' actual:'%c'", k, l)
	}
	if v := n.Value().(int); v != value {
		t.Errorf("TrieNode.Value() expected:%d actual:%d", value, v)
	}
}

func TestTrie(t *testing.T) {
	trie := NewTrie()
	for i := 1; i <= 5; i++ {
		trie.Put(KeyList{Key{0, 0, rune(i)}}, 111*i)
	}

	nodes := Children(trie.Root())
	for i := 0; i < 5; i++ {
		checkTrieNode(t, nodes[i], Key{0, 0, rune(i + 1)}, 111*(i+1))
	}

	if s := trie.Size(); s != 5 {
		t.Errorf("trie.Size() returns not 5: %d", s)
	}
}

func TestNotFound(t *testing.T) {
	trie := NewTrie()
	if trie.Get(Key{999, 999, 'a'}) != nil {
		t.Errorf("found 'not_exist' in empty trie")
	}
}
