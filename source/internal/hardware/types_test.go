package hardware

import (
	"context"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	m, e := Parse([]byte(`{"cpu":{"name":"处理器","status":"ok","items":[{"Name":"测试 CPU"}]}}`))
	if e != nil || m["cpu"].Items[0]["Name"] != "测试 CPU" {
		t.Fatal(e, m)
	}
	if _, e = Parse([]byte("invalid")); e == nil {
		t.Fatal("invalid JSON accepted")
	}
}
func TestRedactionWithoutMutation(t *testing.T) {
	h := Base()
	h.Groups["test"] = Group{Items: []map[string]any{{"SerialNumber": "secret", "UUID": "secret", "IdentifyingNumber": "secret", "Name": "visible"}}}
	v := Redact(h)
	for _, key := range []string{"SerialNumber", "UUID", "IdentifyingNumber"} {
		if v.Groups["test"].Items[0][key] != "[已隐藏]" {
			t.Fatal(key)
		}
	}
	if h.Groups["test"].Items[0]["SerialNumber"] != "secret" {
		t.Fatal("redaction mutated cache")
	}
}
func TestCacheCancelAndJoin(t *testing.T) {
	c := NewCache()
	ctx, cancel := context.WithCancel(context.Background())
	c.Refresh(ctx)
	cancel()
	c.Cancel()
	done := make(chan struct{})
	go func() { c.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("cache did not join cancelled query")
	}
}
