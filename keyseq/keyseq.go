package keyseq

import (
	"sync"
	"time"

	"github.com/nsf/termbox-go"
)

// Key is data in one trie node in the KeySequence
type Key struct {
	Modifier int // Alt, etc
	Key      termbox.Key
	Ch       rune
}

func NewKeyFromKey(k termbox.Key) Key {
	return Key{0, k, rune(0)}
}

// KeyList is just the list of keys
type KeyList []Key

func (k Key) Compare(x Key) int {
	if k.Modifier < x.Modifier {
		return -1
	} else if k.Modifier > x.Modifier {
		return 1
	}

	if k.Key < x.Key {
		return -1
	} else if k.Key > x.Key {
		return 1
	}

	if k.Ch < x.Ch {
		return -1
	} else if k.Ch > x.Ch {
		return 1
	}

	return 0
}

func (k KeyList) Equals(x KeyList) bool {
	if len(k) != len(x) {
		return false
	}

	for i := 0; i < len(k); i++ {
		if k[i].Compare(x[i]) != 0 {
			return false
		}
	}
	return true
}

type keyseqMatcher interface {
	Get(Key) Node
	GetList(KeyList) Node
}

type Keyseq struct {
	*Matcher
	current       keyseqMatcher
	mutex         *sync.Mutex
	prevInputTime time.Time
}

func New() *Keyseq {
	return &Keyseq{NewMatcher(), nil, &sync.Mutex{}, time.Time{}}
}

func (k *Keyseq) SetCurrent(m keyseqMatcher) {
	k.current = m
}

func (k *Keyseq) Current() keyseqMatcher {
	if k.current == nil {
		k.current = k.Matcher
	}
	return k.current
}

func (k *Keyseq) AcceptKey(key Key) interface{} {
	k.mutex.Lock()
	defer k.mutex.Unlock()
	defer func() { k.prevInputTime = time.Now() }()
	c := k.Current()
	n := c.Get(key)

	// nothing matched
	if n == nil {
		k.SetCurrent(k.Matcher)
		return nil
	}

	// Matched node has children. It MAY BE a part of a key sequence
	if n.HasChildren() {
		// Did we get this input in succession, i.e. withing 200 msecs?
		// if yes, we may need to check for key sequence
		if time.Since(k.prevInputTime) <= time.Second {
			k.SetCurrent(n)
			// if this is in the middle of a sequence, nodeData contains
			// nothing. in that case we should just return what the default
			// (top-level) key action would have been
			x := n.Value().(*nodeData).Value()
			if x != nil {
				return x
			}
		}
		n = k.Matcher.Get(key)
		if n == nil || n.Value() == nil {
			// Nothing matched, but we may be in the middle of a sequence,
			// so don't reset the current node
			return nil
		}

		// Something matched, return it
		return n.Value().(*nodeData).Value()
	}

	// If it got here, we should just rest the matcher, and return
	// whatever we matched
	k.SetCurrent(k.Matcher)
	return n.Value().(*nodeData).Value()
}