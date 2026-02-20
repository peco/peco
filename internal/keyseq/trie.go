package keyseq

import (
	"container/list"
)

type Trie interface {
	Root() Node
	GetList(KeyList) Node
	Get(Key) Node
	Put(KeyList, any) Node
	Size() int
}

// NewTrie creates a new empty Trie for storing key sequences.
func NewTrie() Trie {
	return NewTernaryTrie()
}

// Get looks up a value by key sequence path in the trie, returning nil if not found.
func Get(t Trie, k KeyList) Node {
	if t == nil {
		return nil
	}
	n := t.Root()
	for _, c := range k {
		n = n.Get(c)
		if n == nil {
			return nil
		}
	}
	return n
}

// Put inserts a value at the given key sequence path in the trie, creating nodes as needed.
func Put(t Trie, k KeyList, v any) Node {
	if t == nil {
		return nil
	}
	n := t.Root()
	for _, c := range k {
		n, _ = n.Dig(c)
	}
	n.SetValue(v)
	return n
}

// EachDepth iterates over trie nodes in depth-first order, calling proc for each node.
func EachDepth(t Trie, proc func(Node) bool) {
	if t == nil {
		return
	}
	r := t.Root()
	var f func(Node) bool
	f = func(n Node) bool {
		n.Each(f)
		return proc(n)
	}
	r.Each(f)
}

// EachWidth iterates over trie nodes in breadth-first order, calling proc for each node.
func EachWidth(t Trie, proc func(Node) bool) {
	if t == nil {
		return
	}
	q := list.New()
	q.PushBack(t.Root())
	for q.Len() != 0 {
		f := q.Front()
		q.Remove(f)
		t, ok := f.Value.(Node)
		if !ok {
			break
		}
		if !proc(t) {
			break
		}
		t.Each(func(n Node) bool {
			q.PushBack(n)
			return true
		})
	}
}

type Node interface {
	Get(k Key) Node
	GetList(k KeyList) Node
	Dig(k Key) (Node, bool)
	HasChildren() bool
	Size() int
	Each(func(Node) bool)
	RemoveAll()

	Label() Key
	Value() any
	SetValue(v any)
}

// Children returns all child nodes of n as a slice.
func Children(n Node) []Node {
	children := make([]Node, n.Size())
	idx := 0
	n.Each(func(n Node) bool {
		children[idx] = n
		idx++
		return true
	})
	return children
}
