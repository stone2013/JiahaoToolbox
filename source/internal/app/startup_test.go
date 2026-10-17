package app

import (
	"jiahao-toolbox/internal/native"
	"strings"
	"testing"
)

func TestInitialHTMLUsesSavedThemeAndMotion(t *testing.T) {
	a := testApp(t)
	s := a.Config.Get()
	s.Theme = "light"
	s.Animations = false
	if err := a.Config.Update(s); err != nil {
		t.Fatal(err)
	}
	w := request(a, "GET", "/", "", true, false)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `data-theme="light"`) || !strings.Contains(w.Body.String(), `data-motion="off"`) {
		t.Fatal("saved preference missing from first HTML", w.Code)
	}
	if !strings.Contains(w.Body.String(), "startup-background") || !strings.Contains(w.Body.String(), `id="startup"`) {
		t.Fatal("first paint startup missing")
	}
	_ = native.ReducedMotion()
}
