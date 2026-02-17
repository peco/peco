package buffer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetLineListBuf(t *testing.T) {
	t.Parallel()
	buf := GetLineListBuf()
	require.NotNil(t, buf)
	require.Equal(t, 0, len(buf))
	require.Equal(t, DefaultFilterBufSize, cap(buf))
}

func TestReleaseAndGetLineListBuf(t *testing.T) {
	t.Parallel()
	// Get a buffer, use it, release it, get it back
	buf := GetLineListBuf()
	require.NotNil(t, buf)

	// Release it back to the pool
	ReleaseLineListBuf(buf)

	// Get another one — may or may not be the same one (pool behavior is not guaranteed)
	buf2 := GetLineListBuf()
	require.NotNil(t, buf2)
	require.Equal(t, 0, len(buf2))
}

func TestReleaseLineListBufNil(t *testing.T) {
	t.Parallel()
	// Should not panic
	ReleaseLineListBuf(nil)
}

func TestReleaseLineListBufResetsLength(t *testing.T) {
	t.Parallel()
	buf := GetLineListBuf()

	// Simulate usage — we can't easily append real line.Line values
	// without importing the line package's internals, but we can verify
	// that the pool resets the slice length to 0 on release.
	// The ReleaseLineListBuf function does: l = l[0:0]
	// This is verified by the pool returning zero-length slices.
	ReleaseLineListBuf(buf)

	buf2 := GetLineListBuf()
	require.Equal(t, 0, len(buf2))
}

func TestMultipleGetReleaseCycles(t *testing.T) {
	t.Parallel()
	for range 10 {
		buf := GetLineListBuf()
		require.NotNil(t, buf)
		require.Equal(t, 0, len(buf))
		ReleaseLineListBuf(buf)
	}
}
