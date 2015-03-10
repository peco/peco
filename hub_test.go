package peco

import (
	"testing"
	"time"
)

func TestHub(t *testing.T) {
	h := NewHub(5)

	done := make(map[string]time.Time)

	go func() {
		hr := <-h.QueryCh()
		time.Sleep(100 * time.Millisecond)
		done["query"] = time.Now()
		hr.Done()
	}()
	go func() {
		hr := <-h.DrawCh()
		if hr.DataInterface() != nil {
			t.Errorf("Expected data to be nil, got %s", hr.DataInterface())
		}
		time.Sleep(100 * time.Millisecond)
		done["draw"] = time.Now()
		hr.Done()
	}()
	go func() {
		hr := <-h.StatusMsgCh()
		r := hr.DataInterface().(StatusMsgRequest)
		if r.message != "Hello, World!" {
			t.Errorf("Expected data to be 'Hello World!', got '%s'", r.message)
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
		h.SendDraw(nil)
		h.SendStatusMsg("Hello, World!")
		h.SendPaging(ToLineAbove)
	})

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

	h.Stop()

	_, ok := <-h.LoopCh()
	if ok {
		t.Errorf("LoopCh should be closed, but it is not")
	}
}
