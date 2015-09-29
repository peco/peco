package keyseq

type Matcher struct {
	*TernaryTrie
}

type Match struct {
	Index   int
	Pattern KeyList
	Value   interface{}
}

type nodeData struct {
	pattern *KeyList
	value   interface{}
	failure *TernaryNode
}

func (n *nodeData) Value() interface{} {
	return n.value
}

func NewMatcher() *Matcher {
	return &Matcher{
		NewTernaryTrie(),
	}
}

func (m *Matcher) Clear() {
	m.Root().RemoveAll()
}

func (m *Matcher) Add(pattern KeyList, v interface{}) {
	m.Put(pattern, &nodeData{
		pattern: &pattern,
		value:   v,
	})
}

func (m *Matcher) Compile() error {
	m.Balance()
	root := m.Root().(*TernaryNode)
	root.SetValue(&nodeData{failure: root})
	// fill data.failure of each node.
	EachWidth(m, func(n Node) bool {
		parent := n.(*TernaryNode)
		parent.Each(func(m Node) bool {
			fillFailure(m.(*TernaryNode), root, parent)
			return true
		})
		return true
	})
	return nil
}

func fillFailure(curr, root, parent *TernaryNode) {
	data := getNodeData(curr)
	if data == nil {
		data = &nodeData{}
		curr.SetValue(data)
	}
	if parent == root {
		data.failure = root
		return
	}
	// Determine failure node.
	fnode := getNextNode(getNodeFailure(parent, root), root, curr.Label())
	data.failure = fnode
}

func (m *Matcher) Match(text KeyList) <-chan Match {
	ch := make(chan Match, 1)
	go m.startMatch(text, ch)
	return ch
}

func (m *Matcher) startMatch(text KeyList, ch chan<- Match) {
	defer close(ch)
	root := m.Root().(*TernaryNode)
	curr := root
	for i, r := range text {
		curr = getNextNode(curr, root, r)
		if curr == root {
			continue
		}
		fireAll(curr, root, ch, i)
	}
}

func getNextNode(node, root *TernaryNode, r Key) *TernaryNode {
	for {
		next, _ := node.Get(r).(*TernaryNode)
		if next != nil {
			return next
		} else if node == root {
			return root
		}
		node = getNodeFailure(node, root)
	}
}

func fireAll(curr, root *TernaryNode, ch chan<- Match, idx int) {
	for curr != root {
		data := getNodeData(curr)
		if data.pattern != nil {
			ch <- Match{
				Index:   idx - len(*data.pattern) + 1,
				Pattern: *data.pattern,
				Value:   data.value,
			}
		}
		curr = data.failure
	}
}

func getNodeData(node *TernaryNode) *nodeData {
	d, _ := node.Value().(*nodeData)
	return d
}

func getNodeFailure(node, root *TernaryNode) *TernaryNode {
	next := getNodeData(node).failure
	if next == nil {
		return root
	}
	return next
}
