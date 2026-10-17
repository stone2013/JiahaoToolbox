package app

import (
	"encoding/json"
	updater "jiahao-toolbox/internal/update"
	"os"
	"path/filepath"
)

func (a *App) loadUpdateState() {
	b, e := os.ReadFile(filepath.Join(a.DataDir, "update-check.json"))
	if e != nil {
		return
	}
	var info updater.Info
	if json.Unmarshal(b, &info) != nil || info.CurrentVersion == "" || info.ReleasePage != updater.ReleasesPage && len(info.ReleasePage) > len(updater.ReleasesPage)+128 {
		return
	}
	a.updateState = info
}
func (a *App) saveUpdateState(info updater.Info) {
	b, e := json.MarshalIndent(info, "", "  ")
	if e != nil {
		return
	}
	path := filepath.Join(a.DataDir, "update-check.json")
	tmp, e := os.CreateTemp(a.DataDir, ".update-check-*.tmp")
	if e != nil {
		return
	}
	name := tmp.Name()
	if _, e = tmp.Write(b); e == nil {
		e = tmp.Sync()
	}
	ce := tmp.Close()
	if e == nil {
		e = ce
	}
	if e == nil {
		e = os.Rename(name, path)
	}
	if e != nil {
		_ = os.Remove(name)
	}
}
