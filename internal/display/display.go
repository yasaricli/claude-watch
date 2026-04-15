// Package display handles terminal rendering: ANSI colors, box-drawing tables,
// and atomic screen updates for flicker-free watch mode.
package display

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yasaricli/claude-watch/internal/parser"
	"github.com/yasaricli/claude-watch/internal/runner"
)

// ── Output buffer ────────────────────────────────────────────────────────────

var buf bytes.Buffer
var watchReady bool

func printf(format string, args ...any) { fmt.Fprintf(&buf, format, args...) }
func println(args ...any)               { fmt.Fprintln(&buf, args...) }

// Flush writes the buffered frame to stdout. In watch mode it moves the
// cursor to the top-left corner so the frame is overwritten in-place.
func Flush(watchMode bool) {
	if watchMode {
		if !watchReady {
			os.Stdout.WriteString("\033[2J\033[H")
			watchReady = true
		} else {
			os.Stdout.WriteString("\033[H")
		}
	}
	os.Stdout.Write(buf.Bytes())
	if watchMode {
		os.Stdout.WriteString("\033[J")
	}
	buf.Reset()
}

// ── ANSI helpers ──────────────────────────────────────────────────────────────

const (
	reset    = "\033[0m"
	bold     = "\033[1m"
	dim      = "\033[2m"
	cyan     = "\033[36m"
	green    = "\033[32m"
	yellow   = "\033[33m"
	magenta  = "\033[35m"
	white    = "\033[97m"
)

func c(color, text string) string {
	if !isTerm() {
		return text
	}
	return color + text + reset
}

func isTerm() bool {
	fi, err := os.Stdout.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// visibleLen counts printable runes, skipping ANSI escape sequences.
func visibleLen(s string) int {
	n, inEsc := 0, false
	for _, r := range s {
		if inEsc {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEsc = false
			}
			continue
		}
		if r == '\033' {
			inEsc = true
			continue
		}
		n++
	}
	return n
}

func pad(s string, w int) string {
	if v := visibleLen(s); v < w {
		return s + strings.Repeat(" ", w-v)
	}
	return s
}

func rpad(s string, w int) string {
	if v := visibleLen(s); v < w {
		return strings.Repeat(" ", w-v) + s
	}
	return s
}

func compact(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1e3)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// ── Box drawing ───────────────────────────────────────────────────────────────

const (
	tl = "╭"; tr = "╮"; bl = "╰"; br = "╯"
	h  = "─"; v  = "│"
	ml = "├"; mr = "┤"; tm = "┬"; bm = "┴"; x = "┼"
)

// Column widths (visible characters, excluding cell padding).
const (
	wName   = 32
	wReq    = 6
	wIn     = 7
	wOut    = 7
	wStatus = 10
)

var widths = []int{wName, wReq, wIn, wOut, wStatus}

func hline(l, m, r string) string {
	parts := make([]string, len(widths))
	for i, w := range widths {
		parts[i] = strings.Repeat(h, w+2)
	}
	return l + strings.Join(parts, m) + r
}

func cell(s string, w int, right bool) string {
	if right {
		return " " + rpad(s, w) + " "
	}
	return " " + pad(s, w) + " "
}

// ── Public API ────────────────────────────────────────────────────────────────

// ProjectRow is one row in the "all projects" table.
type ProjectRow struct {
	Name   string
	Stats  *parser.Stats
	Active bool
}

// SessionTable renders per-session detail for a single project.
func SessionTable(sessions []*parser.Session, running map[string]*runner.Session, projectName string, watchMode bool) {
	total := parser.NewStats()
	for _, s := range sessions {
		total.Add(s.Stats)
	}

	header(projectName, watchMode)
	tableTop()

	for _, info := range sessions {
		_, active := running[info.ID]
		status := c(dim, "○ closed")
		if active {
			status = c(green, "● active")
		}

		timeStr := ""
		if !info.Stats.LastRequest.IsZero() {
			timeStr = info.Stats.LastRequest.Format("01/02 15:04")
		}

		name := c(yellow, shortID(info.ID)) + "  " + c(dim, timeStr)
		printf("%s%s%s%s%s%s%s%s%s%s%s\n",
			v,
			cell(name, wName, false), v,
			cell(c(white, fmt.Sprintf("%d", info.Stats.Requests)), wReq, true), v,
			cell(c(cyan, compact(info.Stats.InputTokens)), wIn, true), v,
			cell(c(magenta, compact(info.Stats.OutputTokens)), wOut, true), v,
			cell(status, wStatus, false), v,
		)
	}

	tableBottom(total, projectName)
}

// AllTable renders one row per project.
func AllTable(rows []ProjectRow, total *parser.Stats, watchMode bool) {
	header("all projects", watchMode)
	tableTop()

	for _, r := range rows {
		status := ""
		sc := ""
		if r.Active {
			status = "● active"
			sc = green
		}
		printf("%s%s%s%s%s%s%s%s%s%s%s\n",
			v,
			cell(c(yellow, r.Name), wName, false), v,
			cell(c(white, fmt.Sprintf("%d", r.Stats.Requests)), wReq, true), v,
			cell(c(cyan, compact(r.Stats.InputTokens)), wIn, true), v,
			cell(c(magenta, compact(r.Stats.OutputTokens)), wOut, true), v,
			cell(c(sc, status), wStatus, false), v,
		)
	}

	tableBottom(total, fmt.Sprintf("%d projects", len(rows)))
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func header(title string, watchMode bool) {
	ts := ""
	if watchMode {
		ts = c(dim, "  updated "+time.Now().Format("15:04:05"))
	}
	printf("\n  %s%s\n\n", c(bold+cyan, title), ts)
}

func tableTop() {
	println(c(dim, hline(tl, tm, tr)))
	printf("%s%s%s%s%s%s%s%s%s%s%s\n",
		v,
		cell(c(dim, "PROJECT"), wName, false), v,
		cell(c(dim, "REQ"), wReq, true), v,
		cell(c(dim, "INPUT"), wIn, true), v,
		cell(c(dim, "OUTPUT"), wOut, true), v,
		cell(c(dim, "STATUS"), wStatus, false), v,
	)
	println(c(dim, hline(ml, x, mr)))
}

func tableBottom(s *parser.Stats, label string) {
	cache := ""
	if r := s.CacheHitRate(); r > 0 {
		cache = c(yellow, fmt.Sprintf("cache %2.0f%%", r))
	}
	println(c(dim, hline(ml, x, mr)))
	printf("%s%s%s%s%s%s%s%s%s%s%s\n",
		v,
		cell(c(bold+white, label), wName, false), v,
		cell(c(bold+green, fmt.Sprintf("%d", s.Requests)), wReq, true), v,
		cell(c(cyan, compact(s.InputTokens)), wIn, true), v,
		cell(c(magenta, compact(s.OutputTokens)), wOut, true), v,
		cell(cache, wStatus, false), v,
	)
	println(c(dim, hline(bl, bm, br)))
}
