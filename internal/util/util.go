package util

import (
	"errors"
	"regexp"
	"unicode"
)

type fder interface {
	Fd() uintptr
}

// CaseInsensitiveIndexFunc returns a function that matches runes equal to r, ignoring case.
func CaseInsensitiveIndexFunc(r rune) func(rune) bool {
	lr := unicode.ToUpper(r)
	return func(v rune) bool {
		return lr == unicode.ToUpper(v)
	}
}

// CaseInsensitiveIndex returns the byte index of the first rune in s that
// is case-insensitively equal to r. Returns -1 if not found. This avoids
// the closure allocation of CaseInsensitiveIndexFunc + strings.IndexFunc.
func CaseInsensitiveIndex(s string, r rune) int {
	upper := unicode.ToUpper(r)
	for i, c := range s {
		if unicode.ToUpper(c) == upper {
			return i
		}
	}
	return -1
}

// ContainsUpper reports whether the string contains any uppercase letter.
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

// IsIgnorableError checks whether err implements the Ignorable interface and returns true.
func IsIgnorableError(err error) bool {
	for e := err; e != nil; e = errors.Unwrap(e) {
		if v, ok := e.(ignorable); ok {
			return v.Ignorable()
		}
	}
	return false
}

// IsCollectResultsError checks whether err signals that results should be collected.
func IsCollectResultsError(err error) bool {
	for e := err; e != nil; e = errors.Unwrap(e) {
		if v, ok := e.(collectResults); ok {
			return v.CollectResults()
		}
	}
	return false
}

// GetExitStatus extracts the exit status code from an error, returning 1 and false if not found.
func GetExitStatus(err error) (int, bool) {
	for e := err; e != nil; e = errors.Unwrap(e) {
		if ese, ok := e.(exitStatuser); ok {
			return ese.ExitStatus(), true
		}
	}
	return 1, false
}
