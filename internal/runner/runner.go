// Package runner detects which Claude Code sessions are currently running
// by checking ~/.claude/sessions/*.json and probing PIDs.
package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"syscall"
)

// Session represents a running Claude Code process.
type Session struct {
	PID       int    `json:"pid"`
	SessionID string `json:"sessionId"`
	CWD       string `json:"cwd"`
}

// Load reads <claude-dir>/sessions/ and returns sessions with live processes.
// Uses CLAUDE_CONFIG_DIR if set, otherwise ~/.claude.
func Load() map[string]*Session {
	dir := filepath.Join(claudeDir(), "sessions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	m := make(map[string]*Session)
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var s Session
		if json.Unmarshal(data, &s) != nil || s.SessionID == "" || s.PID == 0 {
			continue
		}
		if isAlive(s.PID) {
			m[s.SessionID] = &s
		}
	}
	return m
}

func isAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}

func claudeDir() string {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude")
}
