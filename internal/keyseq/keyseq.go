package keyseq

import (
	"strings"
	"sync"
	"time"

	"github.com/nsf/termbox-go"
	"github.com/pkg/errors"
)

var ErrInSequence = errors.New("expected a key sequence")
var ErrNoMatch = errors.New("could not match key to any action")

type ModifierKey int

const (
	ModNone ModifierKey = iota
	ModAlt
	ModMax
)

// Key is data in one trie node in the KeySequence
type Key struct {
	Modifier ModifierKey // Alt, etc
	Key      termbox.Key
	Ch       rune
}

func (kl KeyList) String() string {
	list := make([]string, len(kl))
	for i := 0; i < len(kl); i++ {
		list[i] = kl[i].String()
	}
	return strings.Join(list, ",")
}

func (m ModifierKey) String() string {
	switch m {
	case ModAlt:
		return "M"
	default:
		return ""
	}
}

func (k Key) String() string {
	var s string
	if m := k.Modifier.String(); m != "" {
		s += m + "-"
	}

	if k.Key == 0 {
		s += string([]rune{k.Ch})
	} else {
		s += keyToString[k.Key]
	}

	return s
}

func NewKeyFromKey(k termbox.Key) Key {
	return Key{
		Modifier: 0,
		Key:      k,
		Ch:       rune(0),
	}
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
	mutex         sync.Mutex
	prevInputTime time.Time
}

func New() *Keyseq {
	return &Keyseq{
		Matcher: NewMatcher(),
		current: nil,
	}
}

func (k *Keyseq) InMiddleOfChain() bool {
	return k.current != nil && k.current != k.Matcher
}

func (k *Keyseq) CancelChain() {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	k.setCurrent(k.Matcher)
}

func (k *Keyseq) setCurrent(m keyseqMatcher) {
	k.current = m
}

func (k *Keyseq) Current() keyseqMatcher {
	if k.current == nil {
		k.current = k.Matcher
	}
	return k.current
}

func (k *Keyseq) updateInputTime() {
	k.prevInputTime = time.Now()
}

func (k *Keyseq) AcceptKey(key Key) (interface{}, error) {
	// XXX should we return Action instead of interface{}?
	k.mutex.Lock()
	defer k.mutex.Unlock()
	defer k.updateInputTime()
	c := k.Current()
	n := c.Get(key)

	// nothing matched
	if n == nil {
		k.setCurrent(k.Matcher)
		return nil, ErrNoMatch
	}

	// Matched node has children. It MAY BE a part of a key sequence,
	// but the longest one ALWAYS wins. So for example, if you had
	// "C-x,C-n" and "C-x" mapped to something, "C-x" alone will never
	// fire any action
	if n.HasChildren() {
		// Set the current matcher to the matched node, so the next
		// AcceptKey matches AFTER the current node
		k.setCurrent(n)
		return nil, ErrInSequence
	}

	// If it got here, we should just rest the matcher, and return
	// whatever we matched
	k.setCurrent(k.Matcher)

	// This case should never be true, but we make sure to check
	// for it in order to avoid the possibility of a crash
	data := n.Value()
	if data == nil {
		return nil, ErrNoMatch
	}
	return data.(*nodeData).Value(), nil
}
