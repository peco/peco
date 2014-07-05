package keyseq

type TernaryTrie struct {
	root TernaryNode
}

func NewTernaryTrie() *TernaryTrie {
	return &TernaryTrie{}
}

func (t *TernaryTrie) Root() Node {
	return &t.root
}

func (t *TernaryTrie) GetList(k KeyList) Node {
	return Get(t, k)
}

func (t *TernaryTrie) Get(k Key) Node {
	return Get(t, KeyList{k})
}

func (t *TernaryTrie) Put(k KeyList, v interface{}) Node {
	return Put(t, k, v)
}

func (t *TernaryTrie) Size() int {
	count := 0
	EachDepth(t, func(Node) bool {
		count++
		return true
	})
	return count
}

func (t *TernaryTrie) Balance() {
	EachDepth(t, func(n Node) bool {
		n.(*TernaryNode).Balance()
		return true
	})
	t.root.Balance()
}

type TernaryNode struct {
	label      Key
	firstChild *TernaryNode
	low, high  *TernaryNode
	value      interface{}
}

func NewTernaryNode(l Key) *TernaryNode {
	return &TernaryNode{label: l}
}

func (n *TernaryNode) GetList(k KeyList) Node {
	return n.Get(k[0])
}

func (n *TernaryNode) Get(k Key) Node {
	curr := n.firstChild
	for curr != nil {
		switch k.Compare(curr.label) {
		case 0: // equal
			return curr
		case -1: // less
			curr = curr.low
		default: //more
			curr = curr.high
		}
	}
	return nil
}

func (n *TernaryNode) Dig(k Key) (node Node, isnew bool) {
	curr := n.firstChild
	if curr == nil {
		n.firstChild = NewTernaryNode(k)
		return n.firstChild, true
	}
	for {
		switch k.Compare(curr.label) {
		case 0:
			return curr, false
		case -1:
			if curr.low == nil {
				curr.low = NewTernaryNode(k)
				return curr.low, true
			}
			curr = curr.low
		default:
			if curr.high == nil {
				curr.high = NewTernaryNode(k)
				return curr.high, true
			}
			curr = curr.high
		}
	}
}

func (n *TernaryNode) FirstChild() *TernaryNode {
	return n.firstChild
}

func (n *TernaryNode) HasChildren() bool {
	return n.firstChild != nil
}

func (n *TernaryNode) Size() int {
	if n.firstChild == nil {
		return 0
	}
	count := 0
	n.Each(func(Node) bool {
		count++
		return true
	})
	return count
}

func (n *TernaryNode) Each(proc func(Node) bool) {
	var f func(*TernaryNode) bool
	f = func(n *TernaryNode) bool {
		if n != nil {
			if !f(n.low) || !proc(n) || !f(n.high) {
				return false
			}
		}
		return true
	}
	f(n.firstChild)
}

func (n *TernaryNode) RemoveAll() {
	n.firstChild = nil
}

func (n *TernaryNode) Label() Key {
	return n.label
}

func (n *TernaryNode) Value() interface{} {
	return n.value
}

func (n *TernaryNode) SetValue(v interface{}) {
	n.value = v
}

func (n *TernaryNode) children() []*TernaryNode {
	children := make([]*TernaryNode, n.Size())
	if n.firstChild == nil {
		return children
	}
	idx := 0
	n.Each(func(child Node) bool {
		children[idx] = child.(*TernaryNode)
		idx++
		return true
	})
	return children
}

func (n *TernaryNode) Balance() {
	if n.firstChild == nil {
		return
	}
	children := n.children()
	for _, child := range children {
		child.low = nil
		child.high = nil
	}
	n.firstChild = balance(children, 0, len(children))
}

func balance(nodes []*TernaryNode, s, e int) *TernaryNode {
	count := e - s
	if count <= 0 {
		return nil
	} else if count == 1 {
		return nodes[s]
	} else if count == 2 {
		nodes[s].high = nodes[s+1]
		return nodes[s]
	} else {
		mid := (s + e) / 2
		n := nodes[mid]
		n.low = balance(nodes, s, mid)
		n.high = balance(nodes, mid+1, e)
		return n
	}
}
