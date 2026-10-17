package diagnostics

import (
	"jiahao-toolbox/internal/hardware"
	"jiahao-toolbox/internal/monitor"
	"math"
	"testing"
	"time"
)

// These fixtures exist only in tests; production has no simulation mode.
func fixture() (monitor.Snapshot, hardware.Inventory, time.Time) {
	now := time.Unix(1700000000, 0)
	s := monitor.Empty()
	s.Time = now.UnixMilli()
	s.Memory = monitor.Memory{Total: 100, Used: 30, Available: 70, Usage: monitor.Valid(30, "%")}
	s.Disks = []monitor.Disk{{ID: "C:", Total: 100, Free: 60, Usage: monitor.Valid(40, "%")}}
	s.Networks = []monitor.Network{{Up: true, Addresses: []string{"192.0.2.1"}}}
	h := hardware.Base()
	h.Status = "ok"
	for _, id := range []string{"system", "cpu", "gpu", "board", "memory", "disk"} {
		h.Groups[id] = hardware.Group{Status: "ok", Items: []map[string]any{{"Name": "unit-test fixture"}}}
	}
	return s, h, now
}
func TestCompleteScore(t *testing.T) {
	s, h, now := fixture()
	r := Run(s, h, now)
	if r.Score == nil || *r.Score != 100 || r.Coverage != 100 || len(r.Checks) != 5 {
		t.Fatal(r)
	}
}
func TestThresholdsAndNonFixedScore(t *testing.T) {
	for _, c := range []struct {
		used   float64
		status string
		deduct int
	}{{84.9, "ok", 0}, {85, "notice", 6}, {94.9, "notice", 6}, {95, "warning", 15}} {
		s, h, now := fixture()
		s.Memory.Usage = monitor.Valid(c.used, "%")
		r := Run(s, h, now)
		if r.Checks[2].Status != c.status || r.Score == nil || *r.Score != 100-c.deduct {
			t.Fatal(c, r)
		}
	}
	for _, c := range []struct {
		free   float64
		status string
		deduct int
	}{{15, "ok", 0}, {14.9, "notice", 8}, {5, "notice", 8}, {4.9, "warning", 18}} {
		s, h, now := fixture()
		s.Disks[0].Usage = monitor.Valid(100-c.free, "%")
		r := Run(s, h, now)
		if r.Checks[1].Status != c.status || r.Score == nil || *r.Score != 100-c.deduct {
			t.Fatal(c, r)
		}
	}
}
func TestInsufficientAndStaleNoScore(t *testing.T) {
	for _, mutate := range []func(*monitor.Snapshot, *hardware.Inventory){func(s *monitor.Snapshot, h *hardware.Inventory) { s.Disks = nil }, func(s *monitor.Snapshot, h *hardware.Inventory) { s.Memory.Usage = monitor.Unavailable("%", "no data") }, func(s *monitor.Snapshot, h *hardware.Inventory) { s.Time -= 16000 }, func(s *monitor.Snapshot, h *hardware.Inventory) { s.Time += 2000 }, func(s *monitor.Snapshot, h *hardware.Inventory) { h.Status = "loading"; delete(h.Groups, "system") }, func(s *monitor.Snapshot, h *hardware.Inventory) { s.Memory.Usage = monitor.Valid(math.NaN(), "%") }} {
		s, h, now := fixture()
		mutate(&s, &h)
		if r := Run(s, h, now); r.Score != nil {
			t.Fatal("score must be withheld", r)
		}
	}
}
func TestPartialCoverage(t *testing.T) {
	s, h, now := fixture()
	h.Status = "failed"
	r := Run(s, h, now)
	if r.Coverage != 80 || r.Score == nil || *r.Score != 100 {
		t.Fatal("4 reliable checks with 80% coverage expected", r)
	}
	s.Networks[0].Up = false
	r = Run(s, h, now)
	if r.Score == nil || *r.Score != 95 {
		t.Fatal("offline notice weighted against 80", r)
	}
}
func TestInvalidPartitionPreventsScore(t *testing.T) {
	s, h, now := fixture()
	s.Disks = append(s.Disks, monitor.Disk{ID: "D:", Usage: monitor.Unavailable("%", "unreadable")})
	if Run(s, h, now).Score != nil {
		t.Fatal("must not ignore unreadable partition")
	}
}
