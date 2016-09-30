package hub

import (
	"testing"
	"time"
)

func TestHub(t *testing.T) {
	h := New(5)

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
		r := hr.Data().(*statusMsgReq)
		if r.Message() != "Hello, World!" {
			t.Errorf("Expected data to be 'Hello World!', got '%s'", r.Message())
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

	h.Batch(func() {
		h.SendQuery("Hello World!")
		h.SendDraw(true)
		h.SendStatusMsg("Hello, World!")
		h.SendPaging(1)
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
