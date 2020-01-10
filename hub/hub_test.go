package hub_test

import (
	"context"
	"testing"
	"time"

	"github.com/peco/peco/hub"
)

func TestHub(t *testing.T) {
	ctx := context.Background()

	h := hub.New(5)

	done := make(map[string]time.Time)

	go func() {
		hr := <-h.QueryCh()
		time.Sleep(100 * time.Millisecond)
		done["query"] = time.Now()
		hr.Done()
	}()
	go func() {
		hr := <-h.DrawCh()
		switch v := hr.Data(); v.(type) {
		case string, bool, nil:
			// OK
		default:
			t.Errorf("Expected data to be nil, got %s", v)
		}
		time.Sleep(100 * time.Millisecond)
		done["draw"] = time.Now()
		hr.Done()
	}()
	go func() {
		hr := <-h.StatusMsgCh()
		data := hr.Data()
		r, ok := data.(hub.StatusMsg)
		if !ok {
			t.Errorf("expected data to be hub.StatusMsg. got '%T'", hr)
			return
		}

		if r.Message() != "Hello, World!" {
			t.Errorf("Expected data to be 'Hello World!', got '%s'", r.Message())
			return
		}
		time.Sleep(100 * time.Millisecond)
		done["status"] = time.Now()
		hr.Done()
	}()
	go func() {
		hr := <-h.PagingCh()
		time.Sleep(100 * time.Millisecond)
		done["paging"] = time.Now()
		hr.Done()
	}()

	h.Batch(ctx, func(ctx context.Context) {
		h.SendQuery(ctx, "Hello World!")
		h.SendDraw(ctx, true)
		h.SendStatusMsg(ctx, "Hello, World!")
		h.SendPaging(ctx, 1)
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
