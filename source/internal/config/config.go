// Package config stores only application preferences, never hardware identifiers.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

type Settings struct {
	Theme                 string `json:"theme"`
	IntervalMS            int    `json:"intervalMS"`
	Animations            bool   `json:"animations"`
	MonitorAnimation      string `json:"monitorAnimation"`
	Units                 string `json:"units"`
	Disk                  string `json:"disk"`
	Interface             string `json:"interface"`
	Width                 int    `json:"width"`
	Height                int    `json:"height"`
	Maximized             bool   `json:"maximized"`
	CheckUpdatesOnStartup bool   `json:"checkUpdatesOnStartup"`
	AutoDownloadUpdates   bool   `json:"autoDownloadUpdates"`
	AllowBetaUpdates      bool   `json:"allowBetaUpdates"`
}

func Defaults() Settings {
	return Settings{Theme: "dark", IntervalMS: 1000, Animations: true, MonitorAnimation: "smooth", Units: "iec", Width: 1536, Height: 1000, CheckUpdatesOnStartup: false, AutoDownloadUpdates: false, AllowBetaUpdates: false}
}
func Validate(s Settings) error {
	if s.AutoDownloadUpdates {
		return errors.New("自动下载暂不可用：尚未配置可信更新签名验证")
	}
	if s.Theme != "dark" && s.Theme != "light" && s.Theme != "system" {
		return errors.New("主题无效")
	}
	if s.IntervalMS != 500 && s.IntervalMS != 1000 && s.IntervalMS != 2000 && s.IntervalMS != 5000 {
		return errors.New("采样间隔必须为 500/1000/2000/5000 毫秒")
	}
	if s.MonitorAnimation != "off" && s.MonitorAnimation != "standard" && s.MonitorAnimation != "smooth" {
		return errors.New("监控动画必须为 off、standard 或 smooth")
	}
	if s.Units != "iec" && s.Units != "si" {
		return errors.New("单位必须为 iec 或 si")
	}
	if s.Width < 480 || s.Width > 7680 || s.Height < 320 || s.Height > 4320 {
		return errors.New("窗口尺寸超出范围")
	}
	if len(s.Disk) > 16 || len(s.Interface) > 256 {
		return errors.New("设备选择超出长度限制")
	}
	return nil
}

type Store struct {
	mu    sync.RWMutex
	path  string
	value Settings
}

func Open(dir string) (*Store, error) {
	s := &Store{path: filepath.Join(dir, "settings.json"), value: Defaults()}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return s, err
	}
	b, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	candidate := Defaults()
	if err = json.Unmarshal(b, &candidate); err != nil {
		return s, err
	}
	if err = Validate(candidate); err != nil {
		return s, err
	}
	s.value = candidate
	return s, nil
}
func (s *Store) Get() Settings { s.mu.RLock(); defer s.mu.RUnlock(); return s.value }
func (s *Store) Update(v Settings) error {
	if e := Validate(v); e != nil {
		return e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	tmp := s.path + ".tmp"
	f, e := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	if _, e = f.Write(b); e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		os.Remove(tmp)
		return e
	}
	if e = os.Rename(tmp, s.path); e != nil {
		os.Remove(tmp)
		return e
	}
	s.value = v
	return nil
}
