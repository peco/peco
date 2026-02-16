package util

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCaseInsensitiveIndexFunc(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		r       rune
		matchR  rune
		matches bool
	}{
		{"same lowercase", 'a', 'a', true},
		{"upper matches lower", 'a', 'A', true},
		{"lower matches upper", 'A', 'a', true},
		{"different chars", 'a', 'b', false},
		{"unicode same", 'ä', 'Ä', true},
		{"digit matches itself", '1', '1', true},
		{"digit vs letter", '1', 'a', false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fn := CaseInsensitiveIndexFunc(tt.r)
			require.Equal(t, tt.matches, fn(tt.matchR))
		})
	}
}

func TestContainsUpper(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"all lowercase", "hello", false},
		{"all uppercase", "HELLO", true},
		{"mixed case", "hEllo", true},
		{"empty string", "", false},
		{"numbers only", "12345", false},
		{"with uppercase at end", "hellO", true},
		{"with uppercase at start", "Hello", true},
		{"unicode lowercase", "こんにちは", false},
		{"special chars", "!@#$%", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expected, ContainsUpper(tt.input))
		})
	}
}

func TestStripANSISequence(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"no ANSI", "hello", "hello"},
		{"empty string", "", ""},
		{"bold", "\x1B[1mhello\x1B[0m", "hello"},
		{"color red", "\x1B[31mred text\x1B[0m", "red text"},
		{"multiple sequences", "\x1B[1m\x1B[31mhello\x1B[0m", "hello"},
		{"color with semicolon", "\x1B[1;31mbold red\x1B[0m", "bold red"},
		{"mixed content", "before\x1B[32mgreen\x1B[0mafter", "beforegreenafter"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expected, StripANSISequence(tt.input))
		})
	}
}

// Mock error types for testing error interface checks

type mockIgnorableError struct {
	ignorable bool
	msg       string
}

func (e *mockIgnorableError) Error() string   { return e.msg }
func (e *mockIgnorableError) Ignorable() bool { return e.ignorable }

type mockCollectResultsError struct {
	collect bool
	msg     string
}

func (e *mockCollectResultsError) Error() string        { return e.msg }
func (e *mockCollectResultsError) CollectResults() bool { return e.collect }

type mockExitStatusError struct {
	status int
	msg    string
}

func (e *mockExitStatusError) Error() string    { return e.msg }
func (e *mockExitStatusError) ExitStatus() int { return e.status }

func TestIsIgnorableError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"ignorable true", &mockIgnorableError{ignorable: true, msg: "test"}, true},
		{"ignorable false", &mockIgnorableError{ignorable: false, msg: "test"}, false},
		{"non-ignorable error", errors.New("plain error"), false},
		{"wrapped ignorable", fmt.Errorf("wrapper: %w", &mockIgnorableError{ignorable: true, msg: "inner"}), true},
		{"wrapped non-ignorable", fmt.Errorf("wrapper: %w", errors.New("plain")), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expected, IsIgnorableError(tt.err))
		})
	}
}

func TestIsCollectResultsError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"collect true", &mockCollectResultsError{collect: true, msg: "test"}, true},
		{"collect false", &mockCollectResultsError{collect: false, msg: "test"}, false},
		{"plain error", errors.New("plain error"), false},
		{"wrapped collect", fmt.Errorf("wrapper: %w", &mockCollectResultsError{collect: true, msg: "inner"}), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expected, IsCollectResultsError(tt.err))
		})
	}
}

func TestGetExitStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		err            error
		expectedStatus int
		expectedFound  bool
	}{
		{"exit status 0", &mockExitStatusError{status: 0, msg: "test"}, 0, true},
		{"exit status 1", &mockExitStatusError{status: 1, msg: "test"}, 1, true},
		{"exit status 42", &mockExitStatusError{status: 42, msg: "test"}, 42, true},
		{"plain error", errors.New("plain"), 1, false},
		{"wrapped exit status", fmt.Errorf("wrapper: %w", &mockExitStatusError{status: 2, msg: "inner"}), 2, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			status, found := GetExitStatus(tt.err)
			require.Equal(t, tt.expectedStatus, status)
			require.Equal(t, tt.expectedFound, found)
		})
	}
}
