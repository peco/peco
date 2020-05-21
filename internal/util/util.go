package util

import (
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

// Function who strips ansi sequences
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

type causer interface {
	Cause() error
}

func IsIgnorableError(e error) bool {
	// Obviously, errors are ignoreable if they are initially nil
	if e == nil {
		return true
	}

	var prev error
	for e != nil {
		switch v := e.(type) {
		case ignorable:
			return v.Ignorable()
		default:
			if v, ok := e.(causer); ok {
				e = v.Cause()
			}

			if prev == e {
				break
			}
		}

		prev = e
	}
	return false
}

func IsCollectResultsError(e error) bool {
	if e == nil {
		return false
	}

	var prev error
	for e != nil {
		switch v := e.(type) {
		case collectResults:
			return v.CollectResults()
		default:
			if v, ok := e.(causer); ok {
				e = v.Cause()
			}

			if prev == e {
				break
			}
		}

		prev = e
	}
	return false
}

func GetExitStatus(e error) (int, bool) {
	if e == nil {
		return 1, false
	}

	var prev error
	for e != nil {
		switch v := e.(type) {
		case exitStatuser:
			return v.ExitStatus(), true
		default:
			if v, ok := e.(causer); ok {
				e = v.Cause()
			}

			if prev == e {
				break
			}
		}

		prev = e
	}

	return 1, false
}
