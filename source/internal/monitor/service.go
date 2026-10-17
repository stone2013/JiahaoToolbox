package monitor

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Ring always owns its array. Snapshots returned by a collector must be immutable.
type Ring struct {
	data         [121]Snapshot
	start, count int
}

func (r *Ring) Add(v Snapshot) {
	i := (r.start + r.count) % len(r.data)
	r.data[i] = v
	if r.count == len(r.data) {
		r.start = (r.start + 1) % len(r.data)
	} else {
		r.count++
	}
}
func (r *Ring) Values() []Snapshot {
	o := make([]Snapshot, 0, r.count)
	for i := 0; i < r.count; i++ {
		o = append(o, r.data[(r.start+i)%len(r.data)])
	}
	return o
}

type State struct {
	Latest     Snapshot   `json:"latest"`
	History    []Snapshot `json:"history"`
	Paused     bool       `json:"paused"`
	Background bool       `json:"background"`
	IntervalMS int        `json:"intervalMS"`
	Samples    uint64     `json:"samples"`
}
type Service struct {
	mu                 sync.RWMutex
	c                  Collector
	ring               Ring
	latest             Snapshot
	interval           time.Duration
	paused, background bool
	wake               chan struct{}
	done               chan struct{}
	once               sync.Once
	running            bool
	samples            uint64
}

func NewService(c Collector, ms int) *Service {
	return &Service{c: c, latest: Empty(), interval: time.Duration(ms) * time.Millisecond, wake: make(chan struct{}, 1), done: make(chan struct{})}
}
func (s *Service) Start(ctx context.Context) {
	s.once.Do(func() { s.mu.Lock(); s.running = true; s.mu.Unlock(); go s.run(ctx) })
}
func (s *Service) run(ctx context.Context) {
	defer close(s.done)
	defer s.c.Close()
	var last time.Time
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.wake:
		case <-timer.C:
		}
		if ctx.Err() != nil {
			return
		}
		s.mu.RLock()
		p, b, d := s.paused, s.background, s.interval
		s.mu.RUnlock()
		if b && d < 5*time.Second {
			d = 5 * time.Second
		}
		wait := d
		if !p {
			if last.IsZero() || time.Since(last) >= d {
				v := safeSample(s.c)
				last = time.Now()
				s.mu.Lock()
				s.latest = v
				s.ring.Add(v)
				s.samples++
				s.mu.Unlock()
			} else {
				wait = d - time.Since(last)
				if wait < time.Millisecond {
					wait = time.Millisecond
				}
			}
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(wait)
	}
}
func (s *Service) Configure(ms int, paused, background bool) {
	if ms != 500 && ms != 1000 && ms != 2000 && ms != 5000 {
		ms = 1000
	}
	s.mu.Lock()
	if s.interval == time.Duration(ms)*time.Millisecond && s.paused == paused && s.background == background {
		s.mu.Unlock()
		return
	}
	s.interval = time.Duration(ms) * time.Millisecond
	s.paused = paused
	s.background = background
	s.mu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
}
func (s *Service) State(history bool) State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v := State{Latest: s.latest, Paused: s.paused, Background: s.background, IntervalMS: int(s.interval / time.Millisecond), Samples: s.samples}
	if history {
		v.History = s.ring.Values()
	}
	return v
}
func (s *Service) Wait() {
	s.mu.RLock()
	started := s.running
	s.mu.RUnlock()
	if started {
		<-s.done
	}
}

// Recover a Go-level provider panic without inventing successful readings. This
// cannot interrupt a blocking Windows kernel/driver call; that needs real testing.
func safeSample(c Collector) (v Snapshot) {
	defer func() {
		if e := recover(); e != nil {
			v = Empty()
			v.Errors = append(v.Errors, fmt.Sprintf("采样服务异常：%v", e))
		}
	}()
	return c.Sample()
}
