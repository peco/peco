package selection

import (
	"sync"
	"testing"
	"time"

	"github.com/peco/peco/line"
	"github.com/stretchr/testify/require"
)

func TestSelection(t *testing.T) {
	s := New()

	var i uint64
	alice := line.NewRaw(i, "Alice", false, false)
	i++
	s.Add(alice)
	require.Equal(t, 1, s.Len())
	s.Add(line.NewRaw(i, "Bob", false, false))
	require.Equal(t, 2, s.Len())
	s.Add(alice)
	require.Equal(t, 2, s.Len())
	s.Remove(alice)
	require.Equal(t, 1, s.Len())
}

func TestSelectionHas(t *testing.T) {
	t.Parallel()
	s := New()
	alice := line.NewRaw(0, "Alice", false, false)
	bob := line.NewRaw(1, "Bob", false, false)

	s.Add(alice)
	require.True(t, s.Has(alice))
	require.False(t, s.Has(bob))
}

func TestSelectionAscendOrder(t *testing.T) {
	t.Parallel()
	s := New()
	s.Add(line.NewRaw(3, "Charlie", false, false))
	s.Add(line.NewRaw(1, "Alice", false, false))
	s.Add(line.NewRaw(2, "Bob", false, false))

	var ids []uint64
	s.Ascend(func(l line.Line) bool {
		ids = append(ids, l.ID())
		return true
	})

	require.Equal(t, []uint64{1, 2, 3}, ids, "Ascend should iterate in ID order")
}

func TestSelectionReset(t *testing.T) {
	t.Parallel()
	s := New()
	s.Add(line.NewRaw(0, "Alice", false, false))
	s.Add(line.NewRaw(1, "Bob", false, false))
	require.Equal(t, 2, s.Len())

	s.Reset()
	require.Equal(t, 0, s.Len())
}

func TestSelectionCopy(t *testing.T) {
	t.Parallel()
	src := New()
	src.Add(line.NewRaw(0, "Alice", false, false))
	src.Add(line.NewRaw(1, "Bob", false, false))

	dst := New()
	src.Copy(dst)

	require.Equal(t, 2, dst.Len())
	require.True(t, dst.Has(line.NewRaw(0, "Alice", false, false)))
	require.True(t, dst.Has(line.NewRaw(1, "Bob", false, false)))
}

func TestSelectionConcurrentAccess(t *testing.T) {
	t.Parallel()
	s := New()

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(2)
		go func(id uint64) {
			defer wg.Done()
			s.Add(line.NewRaw(id, "line", false, false))
		}(uint64(i))
		go func(id uint64) {
			defer wg.Done()
			s.Has(line.NewRaw(id, "line", false, false))
		}(uint64(i))
	}
	wg.Wait()

	require.Equal(t, 50, s.Len())
}

func TestRangeStart(t *testing.T) {
	t.Parallel()
	var rs RangeStart

	require.False(t, rs.Valid())

	rs.SetValue(5)
	require.True(t, rs.Valid())
	require.Equal(t, 5, rs.Value())

	rs.Reset()
	require.False(t, rs.Valid())
}

func TestCopySelf(t *testing.T) {
	s := New()
	s.Add(line.NewRaw(0, "Alice", false, false))
	s.Add(line.NewRaw(1, "Bob", false, false))

	done := make(chan struct{})
	go func() {
		defer close(done)
		s.Copy(s)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Copy(self) deadlocked")
	}

	require.Equal(t, 2, s.Len())
}

func TestCopyCrossNoDeadlock(t *testing.T) {
	a := New()
	b := New()
	a.Add(line.NewRaw(0, "Alice", false, false))
	b.Add(line.NewRaw(1, "Bob", false, false))

	done := make(chan struct{})
	go func() {
		defer close(done)
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			a.Copy(b)
		}()
		go func() {
			defer wg.Done()
			b.Copy(a)
		}()
		wg.Wait()
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("cross-Copy deadlocked")
	}
}
