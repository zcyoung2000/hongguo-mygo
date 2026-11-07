package uiapp

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const defaultSidebarWidth float32 = 210

type uiPreferences struct {
	SidebarWidth float32 `json:"sidebarWidth"`
}

func (a *App) LoadPreferences(path string) {
	value := uiPreferences{SidebarWidth: defaultSidebarWidth}
	data, err := os.ReadFile(path)
	if err == nil {
		err = json.Unmarshal(data, &value)
	}
	a.mu.Lock()
	a.preferencesPath = path
	a.preferencesLoaded = err == nil
	a.sidebarWidth = clampSidebar(value.SidebarWidth)
	a.mu.Unlock()
}
func clampSidebar(value float32) float32 {
	if value < 160 {
		return 160
	}
	if value > 420 {
		return 420
	}
	return value
}
func (a *App) SetSidebarWidth(value float32) {
	a.mu.Lock()
	a.sidebarWidth = clampSidebar(value)
	a.preferencesLoaded = true
	a.invalidate()
	a.preferencesPending.Add(1)
	a.mu.Unlock()
	go func() { defer a.preferencesPending.Done(); a.savePreferences() }()
}
func (a *App) savePreferences() {
	a.preferencesMu.Lock()
	defer a.preferencesMu.Unlock()
	a.mu.Lock()
	value, path := uiPreferences{a.sidebarWidth}, a.preferencesPath
	a.mu.Unlock()
	if path == "" {
		return
	}
	data, err := json.Marshal(value)
	if err != nil {
		return
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".ui-prefs-*")
	if err != nil {
		return
	}
	name := file.Name()
	defer os.Remove(name)
	if _, err = file.Write(data); err != nil {
		file.Close()
		return
	}
	if file.Close() == nil {
		_ = os.Rename(name, path)
	}
}
func (s *NativeShell) ImportSidebarWidth(value float32) {
	s.App.mu.Lock()
	loaded := s.App.preferencesLoaded
	s.App.mu.Unlock()
	if !loaded && value >= 160 && value <= 420 {
		s.App.SetSidebarWidth(value)
	}
}

func (a *App) FlushPreferences() { a.preferencesPending.Wait() }
