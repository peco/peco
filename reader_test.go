package peco

import "testing"

// some little test to validate ansi strips function
func TestAnsiStrips(t *testing.T) {
	test := StripsAnsiSequences("this is not a pipe")
	if test != "this is not a pipe" {
		t.Errorf("expected String = 'this is not a pipe', got '%s'", test)
	}
	test = StripsAnsiSequences(" [01;34helloWorld [0m")
	if test != " [01;34helloWorld [0m" {
		t.Errorf("expected String = ' [01;34mhelloWorld [0m', got '%s'", test)
	}
	test = StripsAnsiSequences("\x1b[01;34mthe answer to life is \x1b[0;42m42")
	if test != "the answer to life is 42" {
		t.Errorf("expected String = 'the answer to life is 42' , got '%s'", test)
	}
	test = StripsAnsiSequences("x1b[01;34mthe answer to life is x1b[0;42m42")
	if test != "x1b[01;34mthe answer to life is x1b[0;42m42" {
		t.Errorf("expected String = 'x1b[01;34mthe answer to life is x1b[0;42m42' , got '%s'", test)
	}
}
