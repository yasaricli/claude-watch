// Package parser reads Claude Code session JSONL files and extracts
// API request counts, token usage, and estimated costs.
package parser

import (
	"bufio"
	"encoding/json"
	"os"
	"time"
)

// Entry represents a single line in a Claude Code session JSONL file.
type Entry struct {
	Type      string `json:"type"`
	UUID      string `json:"uuid"`
	Timestamp string `json:"timestamp"`
	SessionID string `json:"sessionId"`
	Message   struct {
		Model string `json:"model"`
		Usage Usage  `json:"usage"`
	} `json:"message"`
}

// Usage holds token consumption data from one API response.
type Usage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	ServerToolUse            struct {
		WebSearchRequests int `json:"web_search_requests"`
		WebFetchRequests  int `json:"web_fetch_requests"`
	} `json:"server_tool_use"`
}

// Stats holds aggregated token and request counts.
type Stats struct {
	Requests                 int
	InputTokens              int
	OutputTokens             int
	CacheCreationInputTokens int
	CacheReadInputTokens     int
	WebSearchRequests        int
	WebFetchRequests         int
	Models                   map[string]int
	FirstRequest             time.Time
	LastRequest              time.Time
}

// NewStats returns a zero-valued Stats.
func NewStats() *Stats {
	return &Stats{Models: make(map[string]int)}
}

// Add merges another Stats into this one.
func (s *Stats) Add(other *Stats) {
	s.Requests += other.Requests
	s.InputTokens += other.InputTokens
	s.OutputTokens += other.OutputTokens
	s.CacheCreationInputTokens += other.CacheCreationInputTokens
	s.CacheReadInputTokens += other.CacheReadInputTokens
	s.WebSearchRequests += other.WebSearchRequests
	s.WebFetchRequests += other.WebFetchRequests
	for m, c := range other.Models {
		s.Models[m] += c
	}
	if !other.FirstRequest.IsZero() {
		if s.FirstRequest.IsZero() || other.FirstRequest.Before(s.FirstRequest) {
			s.FirstRequest = other.FirstRequest
		}
	}
	if other.LastRequest.After(s.LastRequest) {
		s.LastRequest = other.LastRequest
	}
}

// EstimatedCostUSD returns an approximate USD cost based on
// Claude Sonnet 4.6 public pricing (per 1M tokens):
//
//	Input:      $3.00
//	Output:    $15.00
//	CacheWrite: $3.75
//	CacheRead:  $0.30
func (s *Stats) EstimatedCostUSD() float64 {
	const (
		inputPrice      = 3.00
		outputPrice     = 15.00
		cacheWritePrice = 3.75
		cacheReadPrice  = 0.30
	)
	return float64(s.InputTokens)/1e6*inputPrice +
		float64(s.OutputTokens)/1e6*outputPrice +
		float64(s.CacheCreationInputTokens)/1e6*cacheWritePrice +
		float64(s.CacheReadInputTokens)/1e6*cacheReadPrice
}

// CacheHitRate returns the percentage of tokens served from cache.
func (s *Stats) CacheHitRate() float64 {
	total := s.InputTokens + s.CacheCreationInputTokens + s.CacheReadInputTokens
	if total == 0 {
		return 0
	}
	return float64(s.CacheReadInputTokens) / float64(total) * 100
}

// Session holds parsed data for one session file.
type Session struct {
	ID       string
	FilePath string
	ModTime  time.Time
	Stats    *Stats
}

// ParseSession reads a single session JSONL file and returns aggregated stats.
func ParseSession(filePath string) (*Session, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fi, _ := f.Stat()
	var modTime time.Time
	if fi != nil {
		modTime = fi.ModTime()
	}

	sess := &Session{
		FilePath: filePath,
		ModTime:  modTime,
		Stats:    NewStats(),
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		var entry Entry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		if sess.ID == "" && entry.SessionID != "" {
			sess.ID = entry.SessionID
		}
		if entry.Type != "assistant" {
			continue
		}
		u := entry.Message.Usage
		if u.InputTokens == 0 && u.OutputTokens == 0 &&
			u.CacheCreationInputTokens == 0 && u.CacheReadInputTokens == 0 {
			continue
		}

		sess.Stats.Requests++
		sess.Stats.InputTokens += u.InputTokens
		sess.Stats.OutputTokens += u.OutputTokens
		sess.Stats.CacheCreationInputTokens += u.CacheCreationInputTokens
		sess.Stats.CacheReadInputTokens += u.CacheReadInputTokens
		sess.Stats.WebSearchRequests += u.ServerToolUse.WebSearchRequests
		sess.Stats.WebFetchRequests += u.ServerToolUse.WebFetchRequests

		if entry.Message.Model != "" {
			sess.Stats.Models[entry.Message.Model]++
		}
		if entry.Timestamp != "" {
			if t, err := time.Parse(time.RFC3339, entry.Timestamp); err == nil {
				if sess.Stats.FirstRequest.IsZero() || t.Before(sess.Stats.FirstRequest) {
					sess.Stats.FirstRequest = t
				}
				if t.After(sess.Stats.LastRequest) {
					sess.Stats.LastRequest = t
				}
			}
		}
	}

	return sess, scanner.Err()
}

// LoadSessions reads all .jsonl files in dir and returns sessions with
// at least one request, sorted by most-recent first.
func LoadSessions(dir string) ([]*Session, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []*Session
	for _, e := range entries {
		if e.IsDir() || len(e.Name()) < 6 || e.Name()[len(e.Name())-6:] != ".jsonl" {
			continue
		}
		s, err := ParseSession(dir + "/" + e.Name())
		if err != nil || s.Stats.Requests == 0 {
			continue
		}
		out = append(out, s)
	}
	return out, nil
}
