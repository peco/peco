package keyseq

import "github.com/nsf/termbox-go"

// Key is data in one trie node in the KeySequence
type Key struct {
	Modifier int // Alt, etc
	Key      termbox.Key
	Ch       rune
}

func NewKeyFromKey(k termbox.Key) Key {
	return Key{0,k,rune(0)}
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