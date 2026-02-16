package util

import (
	"errors"
	"regexp"
	"unicode"
)

type fder interface {
	Fd() uintptr
}

func CaseInsensitiveIndexFunc(r rune) func(rune) bool {
	lr := unicode.ToUpper(r)
	return func(v rune) bool {
		return lr == unicode.ToUpper(v)
	}
}

func ContainsUpper(query string) bool {
	for _, c := range query {
		if unicode.IsUpper(c) {
			return true
		}
	}
	return false
}

// Global var used to strips ansi sequences
var reANSIEscapeChars = regexp.MustCompile("\x1B\\[(?:[0-9]{1,2}(?:;[0-9]{1,2})?)*[a-zA-Z]")

// StripANSISequence strips ANSI escape sequences from the given string
func StripANSISequence(s string) string {
	return reANSIEscapeChars.ReplaceAllString(s, "")
}

type ignorable interface {
	Ignorable() bool
}

type collectResults interface {
	CollectResults() bool
}

type exitStatuser interface {
	ExitStatus() int
}

func IsIgnorableError(err error) bool {
	for e := err; e != nil; e = errors.Unwrap(e) {
		if v, ok := e.(ignorable); ok {
			return v.Ignorable()
		}
	}
	return false
}

func IsCollectResultsError(err error) bool {
	for e := err; e != nil; e = errors.Unwrap(e) {
		if v, ok := e.(collectResults); ok {
			return v.CollectResults()
		}
	}
	return false
}

func GetExitStatus(err error) (int, bool) {
	for e := err; e != nil; e = errors.Unwrap(e) {
		if ese, ok := e.(exitStatuser); ok {
			return ese.ExitStatus(), true
		}
	}
	return 1, false
}
