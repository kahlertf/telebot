package telebot

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testPoller struct {
	updates chan Update
	done    chan struct{}
}

func newTestPoller() *testPoller {
	return &testPoller{
		updates: make(chan Update, 1),
		done:    make(chan struct{}, 1),
	}
}

func (p *testPoller) Poll(b *Bot, updates chan Update, stop chan struct{}) {
	for {
		select {
		case upd := <-p.updates:
			updates <- upd
		case <-stop:
			return
		}
	}
}

func TestMiddlewarePoller(t *testing.T) {
	tp := newTestPoller()
	var ids []int

	pref := defaultSettings()
	pref.Offline = true

	b, err := NewBot(pref)
	if err != nil {
		t.Fatal(err)
	}

	b.Poller = NewMiddlewarePoller(tp, func(u *Update) bool {
		if u.ID > 0 {
			ids = append(ids, u.ID)
			return true
		}

		tp.done <- struct{}{}
		return false
	})

	go func() {
		tp.updates <- Update{ID: 1}
		tp.updates <- Update{ID: 2}
		tp.updates <- Update{ID: 0}
	}()

	go b.Start()
	<-tp.done
	b.Stop()

	assert.Contains(t, ids, 1)
	assert.Contains(t, ids, 2)
}

func TestLongPollerBackoff(t *testing.T) {
	const (
		fail = `{"ok":false,"error_code":500,"description":"fail"}`
		ok   = `{"ok":true,"result":[]}`
	)
	responses := []string{fail, fail, ok, fail}
	requests := make(chan time.Time, len(responses)+2)

	served := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case requests <- time.Now():
		default:
		}
		if served < len(responses) {
			w.Write([]byte(responses[served]))
			served++
		} else {
			w.Write([]byte(fail))
		}
	}))
	defer server.Close()

	b, err := NewBot(Settings{Token: "token", URL: server.URL, Offline: true})
	require.NoError(t, err)

	stop := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		(&LongPoller{}).Poll(b, make(chan Update), stop)
		close(stopped)
	}()

	stamps := make([]time.Time, len(responses)+1)
	for i := range stamps {
		stamps[i] = <-requests
	}
	close(stop)
	<-stopped

	gap := func(i int) time.Duration { return stamps[i+1].Sub(stamps[i]) }
	assert.GreaterOrEqual(t, gap(0), minPollBackoff, "first retry")
	assert.GreaterOrEqual(t, gap(1), 2*minPollBackoff, "doubled retry")
	assert.Less(t, gap(2), minPollBackoff, "no wait after success")
	assert.GreaterOrEqual(t, gap(3), minPollBackoff, "retry after success")
	assert.Less(t, gap(3), 2*minPollBackoff, "reset after success")
}

func TestNextPollBackoff(t *testing.T) {
	backoff := minPollBackoff
	var steps []time.Duration
	for i := 0; i < 8; i++ {
		backoff = nextPollBackoff(backoff)
		steps = append(steps, backoff)
	}

	assert.Equal(t, []time.Duration{
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
		30 * time.Second,
		30 * time.Second,
		30 * time.Second,
	}, steps)
}
