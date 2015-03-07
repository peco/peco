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

func TestNewMatch(t *testing.T) {
	var m Line

	m = NewRawLine("Hello, World!", false)
	if m.Indices() != nil {
		t.Errorf("NoMatch.Indices() must always return nil")
	}

	nullsepCheck := func(buf string, m Line) {
		if m.Buffer() != buf {
			t.Errorf("m.Buffer() should return '%s', got '%s'", buf, m.Buffer())
		}

		if sepLoc := strings.Index(buf, "\000"); sepLoc > -1 {
			if m.DisplayString() != buf[0:sepLoc] {
				t.Errorf("m.DisplayString() should return '%s', got '%s'", buf[0:sepLoc], m.DisplayString())
			}
			if m.Output() != buf[sepLoc+1:] {
				t.Errorf("m.Output() should return '%s', got '%s'", buf[sepLoc+1:], m.Output())
			}
		} else {
			if m.DisplayString() != m.Buffer() {
				t.Errorf("m.DisplayString() should return '%s', got '%s'", m.Buffer(), m.DisplayString())
			}
			if m.Output() != m.Buffer() {
				t.Errorf("m.Output() should return '%s', got '%s'", m.Buffer(), m.Output())
			}
		}
	}

	makeDidMatch := func(buf string) (string, Line) {
		return buf, NewMatchedLine(NewRawLine(buf, true), [][]int{{0, 5}})
	}

	makeNoMatch := func(buf string) (string, Line) {
		return buf, NewRawLine(buf, true)
	}

	nullsepCheck(makeNoMatch("Hello, World!"))
	nullsepCheck(makeNoMatch("Hello, World!\000Hello, peco!"))
	nullsepCheck(makeDidMatch("Hello, World!"))
	nullsepCheck(makeDidMatch("Hello, World!\000Hello, peco!"))
}
