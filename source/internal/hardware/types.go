package hardware

import (
	"context"
	"encoding/json"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

type Group struct {
	Name   string           `json:"name"`
	Status string           `json:"status"`
	Error  string           `json:"error,omitempty"`
	Items  []map[string]any `json:"items"`
}
type Inventory struct {
	Platform     string           `json:"platform"`
	Hostname     string           `json:"hostname"`
	Architecture string           `json:"architecture"`
	Time         int64            `json:"time"`
	Status       string           `json:"status"`
	Groups       map[string]Group `json:"groups"`
}

func Base() Inventory {
	h, _ := os.Hostname()
	return Inventory{Platform: runtime.GOOS, Hostname: h, Architecture: runtime.GOARCH, Status: "loading", Groups: map[string]Group{}, Time: time.Now().UnixMilli()}
}
func Parse(b []byte) (map[string]Group, error) {
	m := map[string]Group{}
	e := json.Unmarshal(b, &m)
	if e != nil {
		return nil, e
	}
	return m, nil
}

// Serial/UUID identifiers are not included by the collector at all. Additional
// defensive redaction keeps future collectors from leaking them in exports.
func Redact(v Inventory) Inventory {
	b, _ := json.Marshal(v)
	var out Inventory
	json.Unmarshal(b, &out)
	for k, g := range out.Groups {
		for _, m := range g.Items {
			for key := range m {
				l := strings.ToLower(key)
				if strings.Contains(l, "serial") || strings.Contains(l, "uuid") || strings.Contains(l, "identifyingnumber") {
					m[key] = "[已隐藏]"
				}
			}
		}
		out.Groups[k] = g
	}
	return out
}

type Cache struct {
	mu     sync.RWMutex
	v      Inventory
	busy   bool
	done   chan struct{}
	cancel context.CancelFunc
}

func NewCache() *Cache          { return &Cache{v: Base()} }
func (c *Cache) Get() Inventory { c.mu.RLock(); defer c.mu.RUnlock(); return c.v }
func (c *Cache) Refresh(ctx context.Context) bool {
	c.mu.Lock()
	if c.busy {
		c.mu.Unlock()
		return false
	}
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.busy = true
	c.done = make(chan struct{})
	c.v.Status = "loading"
	c.mu.Unlock()
	go func() {
		defer cancel()
		v := Read(ctx)
		c.mu.Lock()
		c.v = v
		c.busy = false
		close(c.done)
		c.mu.Unlock()
	}()
	return true
}
func (c *Cache) Wait() {
	c.mu.RLock()
	d := c.done
	c.mu.RUnlock()
	if d != nil {
		<-d
	}
}

func (c *Cache) Cancel() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.busy && c.cancel != nil {
		c.cancel()
		return true
	}
	return false
}
