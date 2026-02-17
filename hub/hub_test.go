package hub_test

import (
	"context"
	"testing"
	"time"

	"github.com/peco/peco/hub"
	"github.com/stretchr/testify/require"
)

func TestHub(t *testing.T) {
	ctx := context.Background()

	h := hub.New(5)

	done := make(map[string]time.Time)

	go func() {
		hr := <-h.QueryCh()
		if hr.Data() != "Hello World!" {
			t.Errorf("Expected query data to be 'Hello World!', got '%s'", hr.Data())
		}
		time.Sleep(100 * time.Millisecond)
		done["query"] = time.Now()
		hr.Done()
	}()
	go func() {
		hr := <-h.DrawCh()
		// Data() returns *hub.DrawOptions directly — no type assertion needed
		_ = hr.Data()
		time.Sleep(100 * time.Millisecond)
		done["draw"] = time.Now()
		hr.Done()
	}()
	go func() {
		hr := <-h.StatusMsgCh()
		// Data() returns hub.StatusMsg directly — no type assertion needed
		r := hr.Data()
		if r.Message() != "Hello, World!" {
			t.Errorf("Expected data to be 'Hello, World!', got '%s'", r.Message())
			return
		}
		time.Sleep(100 * time.Millisecond)
		done["status"] = time.Now()
		hr.Done()
	}()
	go func() {
		hr := <-h.PagingCh()
		// Data() returns hub.PagingRequest directly — no type assertion needed
		r := hr.Data()
		if r.Type() != hub.PagingRequestType(1) {
			t.Errorf("Expected paging type 1, got %d", r.Type())
		}
		time.Sleep(100 * time.Millisecond)
		done["paging"] = time.Now()
		hr.Done()
	}()

	h.Batch(ctx, func(ctx context.Context) {
		h.SendQuery(ctx, "Hello World!")
		h.SendDraw(ctx, &hub.DrawOptions{})
		h.SendStatusMsg(ctx, "Hello, World!", 0)
		h.SendPaging(ctx, hub.PagingRequestType(1))
	}, true)

	phases := []string{
		"query",
		"draw",
		"status",
		"paging",
	}

	max := len(phases) - 1
	for i := range phases {
		if max == i {
			break
		}

		cur := phases[i]
		next := phases[i+1]

		t.Logf("Checkin if %s was fired before %s", cur, next)
		if done[next].Before(done[cur]) {
			t.Errorf("%s executed before %s?!", next, cur)
		}
	}
}

func TestBatchPanicPropagates(t *testing.T) {
	h := hub.New(5)
	ctx := context.Background()

	// A panic inside a Batch callback must propagate to the caller,
	// not be silently swallowed.
	require.Panics(t, func() {
		h.Batch(ctx, func(_ context.Context) {
			panic("bug in callback")
		}, true)
	}, "Batch must not silently swallow panics")
}

func TestBatchPanicReleasesLock(t *testing.T) {
	h := hub.New(5)
	ctx := context.Background()

	// First call: panic inside a locked Batch.
	func() {
		defer func() { recover() }()
		h.Batch(ctx, func(_ context.Context) {
			panic("first call panics")
		}, true)
	}()

	// Second call: if the mutex was not released by the first panic,
	// this will deadlock. Use a timeout to detect that.
	done := make(chan struct{})
	go func() {
		h.Batch(ctx, func(ctx context.Context) {
			// Drain the message so the synchronous send completes.
			go func() {
				p := <-h.QueryCh()
				p.Done()
			}()
			h.SendQuery(ctx, "after panic")
		}, true)
		close(done)
	}()

	select {
	case <-done:
		// success — mutex was properly released
	case <-time.After(2 * time.Second):
		t.Fatal("Batch deadlocked — mutex was not released after panic")
	}
}

func TestSendStatusMsg(t *testing.T) {
	t.Run("zero delay", func(t *testing.T) {
		h := hub.New(5)
		ctx := context.Background()

		go func() {
			h.SendStatusMsg(ctx, "hello", 0)
		}()

		p := <-h.StatusMsgCh()
		defer p.Done()

		require.Equal(t, "hello", p.Data().Message())
		require.Equal(t, time.Duration(0), p.Data().Delay())
	})

	t.Run("non-zero delay", func(t *testing.T) {
		h := hub.New(5)
		ctx := context.Background()

		go func() {
			h.SendStatusMsg(ctx, "temporary", 500*time.Millisecond)
		}()

		p := <-h.StatusMsgCh()
		defer p.Done()

		require.Equal(t, "temporary", p.Data().Message())
		require.Equal(t, 500*time.Millisecond, p.Data().Delay())
	})
}
