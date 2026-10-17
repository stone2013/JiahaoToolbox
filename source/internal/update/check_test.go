package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSemVerPrecedence(t *testing.T) {
	seq := []string{"v1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta", "1.0.0-beta.2", "1.0.0-beta.11", "1.0.0-rc.1", "1.0.0", "1.0.1", "1.1.0", "2.0.0"}
	for i := 1; i < len(seq); i++ {
		a, e := ParseVersion(seq[i-1])
		if e != nil {
			t.Fatal(e)
		}
		b, e := ParseVersion(seq[i])
		if e != nil {
			t.Fatal(e)
		}
		if Compare(a, b) >= 0 {
			t.Fatalf("expected %s < %s", seq[i-1], seq[i])
		}
	}
	if _, e := ParseVersion("1.02.0"); e == nil {
		t.Fatal("accepted leading zero")
	}
	if _, e := ParseVersion("1.0.0-01"); e == nil {
		t.Fatal("accepted numeric prerelease leading zero")
	}
}
func TestCheckUpdateAndNoDowngrade(t *testing.T) {
	body := `{"tag_name":"v4.5.0","name":"test","body":"notes","published_at":"2026-10-01T00:00:00Z","assets":[{"name":"JiahaoToolbox_v4.5.0_Setup_x64.exe","size":123},{"name":"JiahaoToolbox_v4.5.0_Portable.zip","size":456},{"name":"checksums.txt","size":7}]}`
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/releases/latest" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		ioWrite(w, body)
	}))
	defer s.Close()
	c := &Client{HTTP: s.Client(), APIBase: s.URL}
	got, e := c.Check(context.Background(), "4.4.0", false)
	if e != nil {
		t.Fatal(e)
	}
	if !got.HasUpdate || got.LatestVersion != "v4.5.0" || len(got.Assets) != 2 || got.ReleasePage != "https://github.com/stone2013/JiahaoToolbox/releases/tag/v4.5.0" {
		t.Fatalf("bad info: %+v", got)
	}
	same, e := c.Check(context.Background(), "4.5.0", false)
	if e != nil || same.HasUpdate {
		t.Fatalf("no downgrade expected: %+v %v", same, e)
	}
}
func ioWrite(w http.ResponseWriter, s string) { _, _ = w.Write([]byte(s)) }
func TestNoReleaseRateLimitNetworkAndBeta(t *testing.T) {
	t.Run("404 precise user-facing status", func(t *testing.T) {
		s := httptest.NewServer(http.NotFoundHandler())
		defer s.Close()
		c := &Client{HTTP: s.Client(), APIBase: s.URL}
		got, e := c.Check(context.Background(), "4.4.0", false)
		if e != ErrNoRelease || got.Status != "no_release" {
			t.Fatalf("%+v %v", got, e)
		}
	})
	t.Run("rate limited", func(t *testing.T) {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.WriteHeader(403)
		}))
		defer s.Close()
		c := &Client{HTTP: s.Client(), APIBase: s.URL}
		got, e := c.Check(context.Background(), "4.4.0", false)
		if e == nil || got.Status != "rate_limited" || !strings.Contains(e.Error(), "次数") {
			t.Fatalf("%+v %v", got, e)
		}
	})
	t.Run("allow beta picks highest release semver", func(t *testing.T) {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/releases" {
				t.Errorf("path=%s", r.URL.Path)
			}
			ioWrite(w, `[{"tag_name":"v4.4.0","prerelease":false},{"tag_name":"v4.5.0-beta.2","prerelease":true},{"tag_name":"v4.5.0-beta.10","prerelease":true},{"tag_name":"v99.0.0","draft":true}]`)
		}))
		defer s.Close()
		c := &Client{HTTP: s.Client(), APIBase: s.URL}
		got, e := c.Check(context.Background(), "4.4.0", true)
		if e != nil || got.LatestVersion != "v4.5.0-beta.10" || !got.HasUpdate {
			t.Fatalf("%+v %v", got, e)
		}
	})
	t.Run("reject URL override", func(t *testing.T) {
		c := NewClient()
		c.APIBase = "http://example.com"
		if _, e := c.Check(context.Background(), "4.4.0", false); e == nil {
			t.Fatal("accepted arbitrary HTTP server")
		}
	})
	t.Run("network error", func(t *testing.T) {
		c := NewClient()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, e := c.Check(ctx, "4.4.0", false); e == nil {
			t.Fatal("expected cancelled request")
		}
	})
}
