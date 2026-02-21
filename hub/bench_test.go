package hub_test

import (
	"context"
	"testing"

	"github.com/peco/peco/hub"
)

// BenchmarkHubBatch measures the allocation cost of Hub.Batch context setup.
func BenchmarkHubBatch(b *testing.B) {
	h := hub.New(5)
	ctx := context.Background()

	// Drain query channel and call Done() so batch sends unblock
	go func() {
		for p := range h.QueryCh() {
			p.Done()
		}
	}()

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		h.Batch(ctx, func(bctx context.Context) {
			h.SendQuery(bctx, "test")
		})
	}
}
