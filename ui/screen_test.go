//+build none

package peco_test

import (
	"testing"

	"github.com/peco/peco"
	"github.com/stretchr/testify/assert"
)

func TestTerminal(t *testing.T) {
	if testing.Short() {
		return
	}

	term, err := peco.NewTerminal()
	if !assert.NoError(t, err, `failed to instantiate a terminal`) {
		return
	}
	defer term.Close()
}
