package peco

import "testing"

func TestActionNames(t *testing.T) {
	// These names MUST exist
	names := []string{
		"peco.ForwardChar",
		"peco.BackwardChar",
		"peco.ForwardWord",
		"peco.BackwardWord",
		"peco.BeginningOfLine",
		"peco.EndOfLine",
		"peco.EndOfFile",
		"peco.DeleteForwardChar",
		"peco.DeleteBackwardChar",
		"peco.DeleteForwardWord",
		"peco.DeleteBackwardWord",
		"peco.KillEndOfLine",
		"peco.DeleteAll",
		"peco.SelectPreviousPage",
		"peco.SelectNextPage",
		"peco.SelectPrevious",
		"peco.SelectNext",
		"peco.ToggleSelection",
		"peco.ToggleSelectionAndSelectNext",
		"peco.RotateMatcher",
		"peco.Finish",
		"peco.Cancel",
	}
	for _, name := range names {
		if _, ok := nameToActions[name]; !ok {
			t.Errorf("Action %s should exist, but it does not", name)
		}
	}
}
