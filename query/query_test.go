package query

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQuerySetAndString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"ascii", "hello"},
		{"unicode", "こんにちは"},
		{"mixed", "hello世界"},
		{"with spaces", "hello world"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var q Text
			q.Set(tt.input)
			require.Equal(t, tt.input, q.String())
		})
	}
}

func TestQueryLen(t *testing.T) {
	t.Parallel()
	var q Text
	require.Equal(t, 0, q.Len())

	q.Set("hello")
	require.Equal(t, 5, q.Len())

	// Unicode: each character is one rune
	q.Set("こんにちは")
	require.Equal(t, 5, q.Len())
}

func TestQueryReset(t *testing.T) {
	t.Parallel()
	var q Text
	q.Set("hello")
	require.Equal(t, 5, q.Len())

	q.Reset()
	require.Equal(t, 0, q.Len())
	require.Equal(t, "", q.String())
}

func TestQuerySaveAndRestore(t *testing.T) {
	t.Parallel()
	var q Text
	q.Set("original")

	q.SaveQuery()
	// After save, query is cleared
	require.Equal(t, "", q.String())

	// Setting new query while saved
	q.Set("temporary")
	require.Equal(t, "temporary", q.String())

	// Restore brings back the saved query
	q.RestoreSavedQuery()
	require.Equal(t, "original", q.String())
}

func TestQuerySaveAndRestoreEmpty(t *testing.T) {
	t.Parallel()
	var q Text
	// Save an empty query
	q.SaveQuery()
	require.Equal(t, "", q.String())

	q.Set("something")
	q.RestoreSavedQuery()
	require.Equal(t, "", q.String())
}

func TestQueryDeleteRange(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		initial  string
		start    int
		end      int
		expected string
	}{
		{"delete middle", "abcdef", 2, 4, "abef"},
		{"delete from start", "abcdef", 0, 3, "def"},
		{"delete to end", "abcdef", 3, 6, "abc"},
		{"delete all", "abcdef", 0, 6, ""},
		{"delete single char", "abcdef", 2, 3, "abdef"},
		{"start is -1 (no-op)", "abcdef", -1, 3, "abcdef"},
		{"start > end (no-op)", "abcdef", 4, 2, "abcdef"},
		{"end beyond length (clamped)", "abcdef", 4, 100, "abcd"},
		{"unicode delete", "あいうえお", 1, 3, "あえお"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var q Text
			q.Set(tt.initial)
			q.DeleteRange(tt.start, tt.end)
			require.Equal(t, tt.expected, q.String())
		})
	}
}

func TestQueryRuneSlice(t *testing.T) {
	t.Parallel()
	var q Text
	q.Set("hello")

	runes := q.RuneSlice()
	require.Equal(t, []rune("hello"), runes)

	// Verify it's a copy - modifying the returned slice shouldn't affect the query
	runes[0] = 'H'
	require.Equal(t, "hello", q.String())
}

func TestQueryRuneSliceEmpty(t *testing.T) {
	t.Parallel()
	var q Text
	runes := q.RuneSlice()
	require.Empty(t, runes)
}

func TestQueryRuneAt(t *testing.T) {
	t.Parallel()
	var q Text
	q.Set("hello")

	require.Equal(t, 'h', q.RuneAt(0))
	require.Equal(t, 'e', q.RuneAt(1))
	require.Equal(t, 'o', q.RuneAt(4))
}

func TestQueryRuneAtUnicode(t *testing.T) {
	t.Parallel()
	var q Text
	q.Set("あいう")

	require.Equal(t, 'あ', q.RuneAt(0))
	require.Equal(t, 'い', q.RuneAt(1))
	require.Equal(t, 'う', q.RuneAt(2))
}

func TestQueryRuneAtOutOfBounds(t *testing.T) {
	t.Parallel()
	var q Text
	q.Set("hello")

	// Out-of-bounds index returns zero rune without panicking
	require.Equal(t, rune(0), q.RuneAt(5))
	require.Equal(t, rune(0), q.RuneAt(100))

	// Negative index returns zero rune without panicking
	require.Equal(t, rune(0), q.RuneAt(-1))
	require.Equal(t, rune(0), q.RuneAt(-100))

	// Empty query: any index returns zero rune
	var empty Text
	require.Equal(t, rune(0), empty.RuneAt(0))
	require.Equal(t, rune(0), empty.RuneAt(-1))
}

func TestQueryInsertAt(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		initial  string
		ch       rune
		where    int
		expected string
	}{
		{"insert at beginning", "hello", 'X', 0, "Xhello"},
		{"insert at end (append)", "hello", 'X', 5, "helloX"},
		{"insert in middle", "hello", 'X', 2, "heXllo"},
		{"insert into empty", "", 'X', 0, "X"},
		{"insert unicode", "hello", 'あ', 2, "heあllo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var q Text
			q.Set(tt.initial)
			q.InsertAt(tt.ch, tt.where)
			require.Equal(t, tt.expected, q.String())
		})
	}
}

func TestQueryMultipleInserts(t *testing.T) {
	t.Parallel()
	var q Text
	// Build "abc" by inserting one character at a time
	q.InsertAt('a', 0)
	q.InsertAt('b', 1)
	q.InsertAt('c', 2)
	require.Equal(t, "abc", q.String())

	// Insert at beginning
	q.InsertAt('0', 0)
	require.Equal(t, "0abc", q.String())

	// Insert in middle
	q.InsertAt('X', 2)
	require.Equal(t, "0aXbc", q.String())
}

func TestCaretInitialPos(t *testing.T) {
	t.Parallel()
	var c Caret
	require.Equal(t, 0, c.Pos())
}

func TestCaretSetPos(t *testing.T) {
	t.Parallel()
	var c Caret
	c.SetPos(5)
	require.Equal(t, 5, c.Pos())

	c.SetPos(0)
	require.Equal(t, 0, c.Pos())

	c.SetPos(100)
	require.Equal(t, 100, c.Pos())
}

func TestCaretMove(t *testing.T) {
	t.Parallel()
	var c Caret
	c.SetPos(5)

	c.Move(3)
	require.Equal(t, 8, c.Pos())

	c.Move(-2)
	require.Equal(t, 6, c.Pos())

	// Move to negative territory
	c.Move(-10)
	require.Equal(t, -4, c.Pos())
}

func TestCaretMoveFromZero(t *testing.T) {
	t.Parallel()
	var c Caret

	c.Move(1)
	require.Equal(t, 1, c.Pos())

	c.Move(-1)
	require.Equal(t, 0, c.Pos())
}

func TestCaretMultipleMoves(t *testing.T) {
	t.Parallel()
	var c Caret
	for range 10 {
		c.Move(1)
	}
	require.Equal(t, 10, c.Pos())

	for range 5 {
		c.Move(-1)
	}
	require.Equal(t, 5, c.Pos())
}
