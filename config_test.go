package peco

import (
	"encoding/json"
	"testing"
)

func TestReadRC(t *testing.T) {
	txt := `
{
	"Keymap": {
		"C-j": "peco.Finish"
	},
	"Style": {
		"Basic": ["on_default", "default"],
		"Selected": ["underline", "on_cyan", "black"],
		"Query": ["yellow", "bold"]
	}
}
`
	cfg := NewConfig()
	if err := json.Unmarshal([]byte(txt), cfg); err != nil {
		t.Fatalf("Error unmarshaling json: %s", err)
	}
	t.Logf("%#q", cfg)
}
