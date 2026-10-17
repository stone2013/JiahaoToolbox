package monitor

import (
	"context"
	"encoding/json"
	"math"
	"sync/atomic"
	"testing"
	"time"
)

func assertValue(t *testing.T, v Value, want float64) {
	t.Helper()
	if v.Value == nil || math.Abs(*v.Value-want) > 1e-8 {
		t.Fatalf("got %#v want %v", v, want)
	}
}
func TestPercent(t *testing.T) {
	for _, c := range []struct {
		u, t uint64
		want float64
	}{{0, 100, 0}, {25, 100, 25}, {1, 4, 25}, {1 << 50, 1 << 51, 50}, {100, 100, 100}} {
		assertValue(t, Percent(c.u, c.t), c.want)
	}
	if Percent(0, 0).Value != nil || Percent(101, 100).Value != nil {
		t.Fatal("invalid totals accepted")
	}
}
func TestRate(t *testing.T) {
	assertValue(t, Rate(1200, 200, .5), 2000)
	assertValue(t, Rate(1<<54+200, 1<<54, 2), 100)
	assertValue(t, Rate(50, 50, 1), 0)
	for _, v := range []Value{Rate(1, 2, 1), Rate(1, 0, 0), Rate(1, 0, -1), Rate(1, 0, math.NaN())} {
		if v.Value != nil {
			t.Fatal("invalid rate accepted")
		}
	}
}
func TestCPUUsage(t *testing.T) {
	assertValue(t, CPUUsage(140, 180, 120, 100, 100, 100), 60)
	if CPUUsage(1, 0, 0, 0, 0, 0).Value != nil || CPUUsage(0, 0, 0, 1, 1, 1).Value != nil {
		t.Fatal("bad deltas accepted")
	}
}
func TestNullInsteadOfFake(t *testing.T) {
	for _, v := range []Value{Valid(math.NaN(), "%"), Valid(math.Inf(1), "%"), Unsupported("°C", "no driver"), Unavailable("%", "wait")} {
		b, e := json.Marshal(v)
		if e != nil || v.Value != nil || string(b) == "" {
			t.Fatal("invalid value must marshal as null")
		}
	}
	b, e := json.Marshal(Empty())
	if e != nil || len(b) == 0 {
		t.Fatal(e)
	}
}
func TestRingBoundAndOrder(t *testing.T) {
	var r Ring
	for i := 0; i < 10000; i++ {
		r.Add(Snapshot{Time: int64(i)})
	}
	a := r.Values()
	if len(a) != 121 || a[0].Time != 9879 || a[120].Time != 9999 {
		t.Fatalf("ring ordering %d %#v", len(a), a[0])
	}
	a[0].Time = 0
	if r.Values()[0].Time == 0 {
		t.Fatal("returned outer array aliases buffer")
	}
}

type testCollector struct{ calls, active, max, closed atomic.Int32 }

func (c *testCollector) Sample() Snapshot {
	n := c.active.Add(1)
	for old := c.max.Load(); n > old; old = c.max.Load() {
		if c.max.CompareAndSwap(old, n) {
			break
		}
	}
	defer c.active.Add(-1)
	c.calls.Add(1)
	time.Sleep(time.Millisecond)
	return Empty()
}
func (c *testCollector) Close() error { c.closed.Add(1); return nil }
func waitFor(t *testing.T, f func() bool) {
	t.Helper()
	end := time.Now().Add(time.Second)
	for !f() {
		if time.Now().After(end) {
			t.Fatal("timeout")
		}
		time.Sleep(time.Millisecond)
	}
}
func TestServiceSingleWorkerPauseCancel(t *testing.T) {
	c := &testCollector{}
	ctx, cancel := context.WithCancel(context.Background())
	s := NewService(c, 500)
	s.Start(ctx)
	s.Start(ctx)
	waitFor(t, func() bool { return s.State(false).Samples > 0 })
	s.Configure(500, true, false)
	time.Sleep(15 * time.Millisecond)
	n := c.calls.Load()
	time.Sleep(35 * time.Millisecond)
	if c.calls.Load() != n {
		t.Fatal("paused sampler still running")
	}
	s.Configure(500, false, false)
	waitFor(t, func() bool { return c.calls.Load() > n })
	cancel()
	s.Wait()
	if c.closed.Load() != 1 || c.max.Load() != 1 {
		t.Fatal("collector overlap or close count", c.max.Load(), c.closed.Load())
	}
	if s.State(false).History != nil {
		t.Fatal("history was not optional")
	}
}
func TestServiceConcurrentConfiguration(t *testing.T) {
	c := &testCollector{}
	ctx, cancel := context.WithCancel(context.Background())
	s := NewService(c, 500)
	s.Start(ctx)
	for i := 0; i < 100; i++ {
		s.Configure(500, i%2 == 0, i%3 == 0)
		_ = s.State(true)
	}
	cancel()
	s.Wait()
	if c.max.Load() > 1 {
		t.Fatal("overlapping sampling")
	}
}
func TestNoOpConfigurationDoesNotOversample(t *testing.T) {
	c := &testCollector{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := NewService(c, 1000)
	s.Start(ctx)
	waitFor(t, func() bool { return s.State(false).Samples > 0 })
	for i := 0; i < 1000; i++ {
		s.Configure(1000, false, false)
	}
	time.Sleep(30 * time.Millisecond)
	if c.calls.Load() != 1 {
		t.Fatal("window messages triggered extra samples", c.calls.Load())
	}
	cancel()
	s.Wait()
}

type panicCollector struct{}

func (panicCollector) Sample() Snapshot { panic("test failure") }
func (panicCollector) Close() error     { return nil }
func TestProviderPanicIsUnavailable(t *testing.T) {
	s := safeSample(panicCollector{})
	if s.CPU.Usage.Value != nil || len(s.Errors) != 1 {
		t.Fatal("provider panic must return explicit unavailable data")
	}
}
