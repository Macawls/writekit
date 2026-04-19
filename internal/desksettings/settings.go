package desksettings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Settings struct {
	Autostart          bool   `json:"autostart"`
	CloseToTray        bool   `json:"close_to_tray"`
	StartMinimized     bool   `json:"start_minimized"`
	DataDir            string `json:"data_dir"`
	OnboardingComplete bool   `json:"onboarding_complete"`
}

func Default() Settings {
	return Settings{
		Autostart:      false,
		CloseToTray:    true,
		StartMinimized: false,
	}
}

var PickFolder func(title string) (string, error)

type Store struct {
	mu   sync.Mutex
	path string
}

func Open() (*Store, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("user config dir: %w", err)
	}
	base := filepath.Join(dir, "WriteKit")
	if err := os.MkdirAll(base, 0755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", base, err)
	}
	return &Store{path: filepath.Join(base, "settings.json")}, nil
}

func (s *Store) Path() string { return s.path }

func (s *Store) Load() (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *Store) loadLocked() (Settings, error) {
	b, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return Default(), err
	}
	out := Default()
	if len(b) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return Default(), fmt.Errorf("parse settings: %w", err)
	}
	return out, nil
}

func (s *Store) Save(v Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
