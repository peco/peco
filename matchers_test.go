package peco

import (
	"strings"
	"testing"
)

// some little test to validate ansi strips function
func TestANSIColorStrip(t *testing.T) {
	test := stripANSISequence("this is not a pipe")
	if test != "this is not a pipe" {
		t.Errorf("expected String = 'this is not a pipe', got '%s'", test)
	}
	test = stripANSISequence(" [01;34helloWorld [0m")
	if test != " [01;34helloWorld [0m" {
		t.Errorf("expected String = ' [01;34mhelloWorld [0m', got '%s'", test)
	}
	test = stripANSISequence("\x1b[01;34mthe answer to life is \x1b[0;42m42")
	if test != "the answer to life is 42" {
		t.Errorf("expected String = 'the answer to life is 42' , got '%s'", test)
	}
	test = stripANSISequence("x1b[01;34mthe answer to life is x1b[0;42m42")
	if test != "x1b[01;34mthe answer to life is x1b[0;42m42" {
		t.Errorf("expected String = 'x1b[01;34mthe answer to life is x1b[0;42m42' , got '%s'", test)
	}
}

func TestNewNoMatch(t *testing.T) {
	var m *NoMatch

	m = NewNoMatch("Hello, World!", false)
	if m.Indices() != nil {
		t.Errorf("NoMatch.Indices() must always return nil")
	}

	nullsepCheck := func(buf string) {
		m = NewNoMatch(buf, true)
		if m.Buffer() != buf {
			t.Errorf("m.Buffer() should return '%s', got %s", buf, m.Buffer())
		}

		if sepLoc := strings.Index(buf, "\000"); sepLoc > -1 {
			if m.Line() != buf[0:sepLoc] {
				t.Errorf("m.Line() should return '%s', got '%s'", buf[0:sepLoc], m.Line())
			}
			if m.Output() != buf[sepLoc+1:] {
				t.Errorf("m.Output() should return '%s', got '%s'", buf[sepLoc+1:], m.Output())
			}
		} else {
			if m.Line() != m.Buffer() {
				t.Errorf("m.Line() should return '%s', got '%s'", m.Buffer(), m.Line())
			}
			if m.Output() != m.Buffer() {
				t.Errorf("m.Output() should return '%s', got '%s'", m.Buffer(), m.Output())
			}
		}
	}

	nullsepCheck("Hello, World!")
	nullsepCheck("Hello, World!\000Hello, peco!")
}
