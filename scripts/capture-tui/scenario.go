package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Scenario defines a multi-step TUI interaction to capture.
type Scenario struct {
	Name     string            `json:"name"`
	Category string            `json:"category"`
	Tags     []string          `json:"tags,omitempty"`
	Cols     int               `json:"cols,omitempty"`
	Rows     int               `json:"rows,omitempty"`
	Env      map[string]string `json:"env,omitempty"`
	Steps    []Step            `json:"steps"`
}

// Step is a single action in a scenario.
type Step struct {
	// Action is the type of step: type, key, wait, wait_for, snapshot.
	//
	//   type     — write text characters into the PTY
	//   key      — send a named key (Enter, Escape, Ctrl+C, Tab, Up, etc.)
	//   wait     — sleep for a fixed duration (e.g. "2s", "500ms")
	//   wait_for — poll the VT screen until a regex pattern matches
	//   snapshot — capture the current screen state with a name
	Action string `json:"action"`

	// Value depends on Action:
	//   type     → text to type
	//   key      → key name (see keys.go)
	//   wait     → duration string
	//   wait_for → regex pattern to match against screen text
	//   snapshot → snapshot name (used as filename)
	Value string `json:"value"`

	// Timeout is the maximum wait duration for wait_for (default "15s").
	Timeout string `json:"timeout,omitempty"`

	// Note is a human-readable description of what this step observes.
	Note string `json:"note,omitempty"`
}

// Defaults fills in zero values with defaults.
func (s *Scenario) Defaults() {
	if s.Cols == 0 {
		s.Cols = 80
	}
	if s.Rows == 0 {
		s.Rows = 24
	}
}

// Validate checks the scenario for obvious errors.
func (s *Scenario) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("scenario name is required")
	}
	if len(s.Steps) == 0 {
		return fmt.Errorf("scenario %q has no steps", s.Name)
	}
	for i, step := range s.Steps {
		switch step.Action {
		case "type", "key", "wait", "wait_for", "snapshot":
			// valid
		default:
			return fmt.Errorf("scenario %q step %d: unknown action %q", s.Name, i, step.Action)
		}
		if step.Value == "" {
			return fmt.Errorf("scenario %q step %d: value is required for action %q", s.Name, i, step.Action)
		}
	}
	return nil
}

// LoadScenario reads a single scenario JSON file.
func LoadScenario(path string) (*Scenario, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var s Scenario
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	s.Defaults()
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return &s, nil
}

// LoadScenariosFromDir reads all .json files in a directory (non-recursive).
func LoadScenariosFromDir(dir string) ([]*Scenario, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", dir, err)
	}

	var scenarios []*Scenario
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		s, err := LoadScenario(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		scenarios = append(scenarios, s)
	}

	sort.Slice(scenarios, func(i, j int) bool {
		return scenarios[i].Name < scenarios[j].Name
	})
	return scenarios, nil
}

// LoadScenariosRecursive reads all .json files under a directory tree.
func LoadScenariosRecursive(root string) ([]*Scenario, error) {
	var scenarios []*Scenario
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".json") {
			return nil
		}
		s, loadErr := LoadScenario(path)
		if loadErr != nil {
			return loadErr
		}
		scenarios = append(scenarios, s)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(scenarios, func(i, j int) bool {
		return scenarios[i].Name < scenarios[j].Name
	})
	return scenarios, nil
}
