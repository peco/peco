package keyseq

type Matcher struct {
	*TernaryTrie
}

type Match struct {
	Index   int
	Pattern KeyList
	Value   any
}

type nodeData struct {
	pattern *KeyList
	value   any
	failure *TernaryNode
}

func (n *nodeData) Value() any {
	return n.value
}

// NewMatcher creates a new Aho-Corasick matcher for multi-pattern key sequence matching.
func NewMatcher() *Matcher {
	return &Matcher{
		NewTernaryTrie(),
	}
}

// Clear removes all patterns from the matcher, resetting it to empty state.
func (m *Matcher) Clear() {
	m.Root().RemoveAll()
}

// Add inserts a key sequence pattern with an associated value into the matcher.
func (m *Matcher) Add(pattern KeyList, v any) {
	m.Put(pattern, &nodeData{
		pattern: &pattern,
		value:   v,
	})
}

// Compile builds the failure links needed for Aho-Corasick matching after all patterns are added.
func (m *Matcher) Compile() error {
	m.Balance()
	root, _ := m.Root().(*TernaryNode)
	root.SetValue(&nodeData{failure: root})
	// fill data.failure of each node.
	EachWidth(m, func(n Node) bool {
		parent, _ := n.(*TernaryNode)
		parent.Each(func(m Node) bool {
			child, _ := m.(*TernaryNode)
			fillFailure(child, root, parent)
			return true
		})
		return true
	})
	return nil
}

// fillFailure recursively computes the failure link for curr based on its parent's failure chain.
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

// Match tests a key sequence against all compiled patterns and returns matches on a channel.
func (m *Matcher) Match(text KeyList) <-chan Match {
	ch := make(chan Match, 1)
	go m.startMatch(text, ch)
	return ch
}

// startMatch begins a new match attempt from the root of the Aho-Corasick automaton.
func (m *Matcher) startMatch(text KeyList, ch chan<- Match) {
	defer close(ch)
	root, _ := m.Root().(*TernaryNode)
	curr := root
	for i, r := range text {
		curr = getNextNode(curr, root, r)
		if curr == root {
			continue
		}
		fireAll(curr, root, ch, i)
	}
}

// getNextNode follows failure links from node until it finds a child matching r, or returns root.
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

// fireAll emits all pattern matches found at curr by walking the failure chain back to root.
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

// getNodeData extracts the nodeData stored in a TernaryNode's value.
func getNodeData(node *TernaryNode) *nodeData {
	d, _ := node.Value().(*nodeData)
	return d
}

// getNodeFailure returns the failure link for node, falling back to root if none is set.
func getNodeFailure(node, root *TernaryNode) *TernaryNode {
	next := getNodeData(node).failure
	if next == nil {
		return root
	}
	return next
}
