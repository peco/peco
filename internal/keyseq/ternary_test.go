package keyseq

import "testing"

func TestBalance(t *testing.T) {
	trie := NewTernaryTrie()

	list := []Key{}
	for i := 0; i < 15; i++ {
		list = append(list, Key{0, 0, rune(i)})
	}

	for i, k := range list {
		trie.Put(KeyList{k}, i)
	}
	if s := trie.Size(); s != 15 {
		t.Fatalf("Size() returns not 15: %d", s)
	}
	trie.Balance()

	/*
		n8 := trie.Root().(*TernaryNode).firstChild
		checkTrieNode(t, n8, '8', 7)
		n4 := n8.low
		checkTrieNode(t, n4, '4', 3)
		n12 := n8.high
		checkTrieNode(t, n12, 'C', 11)
		n2 := n4.low
		checkTrieNode(t, n2, '2', 1)
		n6 := n4.high
		checkTrieNode(t, n6, '6', 5)
		n10 := n12.low
		checkTrieNode(t, n10, 'A', 9)
		n14 := n12.high
		checkTrieNode(t, n14, 'E', 13)
		n1 := n2.low
		checkTrieNode(t, n1, '1', 0)
		n3 := n2.high
		checkTrieNode(t, n3, '3', 2)
		n5 := n6.low
		checkTrieNode(t, n5, '5', 4)
		n7 := n6.high
		checkTrieNode(t, n7, '7', 6)
		n9 := n10.low
		checkTrieNode(t, n9, '9', 8)
		n11 := n10.high
		checkTrieNode(t, n11, 'B', 10)
		n13 := n14.low
		checkTrieNode(t, n13, 'D', 12)
		n15 := n14.high
		checkTrieNode(t, n15, 'F', 14)
		assertNilBoth(t, n1)
		assertNilBoth(t, n3)
		assertNilBoth(t, n5)
		assertNilBoth(t, n7)
		assertNilBoth(t, n9)
		assertNilBoth(t, n11)
		assertNilBoth(t, n13)
		assertNilBoth(t, n15)
	*/
}
