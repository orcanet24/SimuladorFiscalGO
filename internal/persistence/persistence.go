// Package persistence saves and restores simulator state to disk so counters,
// company data and print history survive restarts.
package persistence

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"simuladorfiscalgo/internal/state"
)

type fileFormat struct {
	SavedAt      string               `json:"saved_at"`
	Brand        string               `json:"brand"`
	Model        string               `json:"model"`
	CommandCount int                  `json:"command_count"`
	History      []state.HistoryEntry `json:"history"`
	*state.State
}

// Save writes the state atomically (temp file + rename).
func Save(path, brand, model string, commandCount int, history []state.HistoryEntry, st *state.State) error {
	if path == "" {
		return nil
	}
	if history == nil {
		history = []state.HistoryEntry{}
	}
	if len(history) > 500 {
		history = history[len(history)-500:]
	}
	f := &fileFormat{
		SavedAt:      time.Now().Format(time.RFC3339Nano),
		Brand:        brand,
		Model:        model,
		CommandCount: commandCount,
		History:      history,
		State:        st,
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", " ")
	if err := enc.Encode(f); err != nil {
		return err
	}
	out := bytes.TrimRight(buf.Bytes(), "\n")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Load restores a previously saved state. Returns true when a file was read.
func Load(path string, st *state.State, commandCount *int, history *[]state.HistoryEntry) bool {
	if path == "" {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	f := fileFormat{State: st}
	if err := json.Unmarshal(data, &f); err != nil {
		return false
	}
	if st.Company.Rif != "" {
		st.Rif = st.Company.Rif
		st.FiscalRegistration = st.Company.Rif
	}
	if !st.DocumentOpen {
		st.CurrentState = "waiting"
		st.CurrentItems = []state.Item{}
		st.DocumentType = ""
	}
	if commandCount != nil {
		*commandCount = f.CommandCount
	}
	if history != nil {
		if f.History == nil {
			f.History = []state.HistoryEntry{}
		}
		*history = f.History
	}
	return true
}
