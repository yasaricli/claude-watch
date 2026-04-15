// claude-watch monitors Claude Code API usage by reading session data
// stored in ~/.claude/projects/. It shows request counts, token usage,
// estimated costs, and active sessions in a terminal table.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yasaricli/claude-watch/internal/display"
	"github.com/yasaricli/claude-watch/internal/parser"
	"github.com/yasaricli/claude-watch/internal/runner"
	"github.com/yasaricli/claude-watch/internal/watcher"
)

const version = "v0.1.0"

func main() {
	var (
		watch   = flag.Bool("w", false, "watch mode — auto-refresh on file changes")
		path    = flag.String("p", "", "path to project directory (default: cwd)")
		all     = flag.Bool("a", false, "show all projects")
		interval = flag.Duration("i", 2*time.Second, "polling interval in watch mode (e.g. 2s, 5s)")
	)
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, `claude-watch — monitor Claude Code API usage

Usage:
  claude-watch          show current project
  claude-watch -a       show all projects
  claude-watch -w       watch mode (auto-refresh)
  claude-watch -w -a    watch all projects

Flags:
`)
		flag.PrintDefaults()
	}
	flag.Parse()

	if *all {
		showAll(*watch, *interval)
		return
	}

	dir, name, err := findProjectDir(*path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "claude-watch: %v\n\nRun from a directory where you've used Claude Code, or use -p /path or -a.\n", err)
		os.Exit(1)
	}

	if *watch {
		watchProject(dir, name, *interval)
	} else {
		renderProject(dir, name, false)
	}
}

// ── Project helpers ───────────────────────────────────────────────────────────

func claudeDir() string {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude")
}

func projectSlug(path string) string {
	return strings.ReplaceAll(path, string(os.PathSeparator), "-")
}

func slugToName(slug string) string {
	home, _ := os.UserHomeDir()
	homeSlug := strings.ReplaceAll(home, "/", "-")
	s := strings.TrimPrefix(slug, homeSlug)
	for _, mid := range []string{
		"-Desktop-PROJECTS", "-Documents-PROJECTS",
		"-Desktop-Projects", "-Documents-Projects",
		"-Projects", "-projects", "-Desktop", "-Documents",
	} {
		if strings.HasPrefix(s, mid) {
			s = strings.TrimPrefix(s, mid)
			break
		}
	}
	s = strings.TrimPrefix(s, "-")
	if s == "" {
		parts := strings.Split(strings.TrimPrefix(slug, "-"), "-")
		return parts[len(parts)-1]
	}
	return s
}

func findProjectDir(cwd string) (dir, name string, err error) {
	if cwd == "" {
		cwd, err = os.Getwd()
		if err != nil {
			return "", "", fmt.Errorf("cannot determine working directory: %w", err)
		}
	}
	projectsBase := filepath.Join(claudeDir(), "projects")
	for {
		slug := projectSlug(cwd)
		candidate := filepath.Join(projectsBase, slug)
		if fi, e := os.Stat(candidate); e == nil && fi.IsDir() {
			return candidate, filepath.Base(cwd), nil
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			break
		}
		cwd = parent
	}
	return "", "", fmt.Errorf("no Claude Code project found for this directory")
}

// ── Rendering ─────────────────────────────────────────────────────────────────

func renderProject(dir, name string, watchMode bool) {
	running := runner.Load()
	sessions, err := parser.LoadSessions(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return
	}
	if len(sessions) == 0 {
		fmt.Printf("\n  No sessions found for %s\n\n", name)
		display.Flush(false)
		return
	}

	display.SessionTable(sessions, running, name, watchMode)

	// Show active sessions in other projects
	for _, rs := range running {
		slug := projectSlug(rs.CWD)
		otherDir := filepath.Join(claudeDir(), "projects", slug)
		if otherDir == dir {
			continue
		}
		fmt.Printf("  %s %s is also active  %s\n",
			"\033[32m●\033[0m",
			"\033[97m"+slugToName(slug)+"\033[0m",
			"\033[2m(use -a to see all)\033[0m",
		)
	}
	fmt.Println()

	display.Flush(watchMode)
}

func showAll(watchMode bool, interval time.Duration) {
	projectsBase := filepath.Join(claudeDir(), "projects")

	render := func() {
		running := runner.Load()
		entries, err := os.ReadDir(projectsBase)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return
		}

		total := parser.NewStats()
		var rows []display.ProjectRow

		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			sessions, err := parser.LoadSessions(filepath.Join(projectsBase, e.Name()))
			if err != nil || len(sessions) == 0 {
				continue
			}

			projectTotal := parser.NewStats()
			active := false
			for _, s := range sessions {
				projectTotal.Add(s.Stats)
				if _, ok := running[s.ID]; ok {
					active = true
				}
			}
			total.Add(projectTotal)
			rows = append(rows, display.ProjectRow{
				Name:   slugToName(e.Name()),
				Stats:  projectTotal,
				Active: active,
			})
		}

		if len(rows) == 0 {
			fmt.Printf("\n  No Claude Code sessions found.\n\n")
		} else {
			display.AllTable(rows, total, watchMode)
		}
		display.Flush(watchMode)
	}

	render()
	if !watchMode {
		return
	}

	dirs := []string{}
	entries, _ := os.ReadDir(projectsBase)
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, filepath.Join(projectsBase, e.Name()))
		}
	}

	w, _ := watcher.New(dirs, interval)
	defer w.Close()

	for range w.Events {
		render()
	}
}

func watchProject(dir, name string, interval time.Duration) {
	renderProject(dir, name, true)
	w, _ := watcher.New([]string{dir}, interval)
	defer w.Close()
	for range w.Events {
		renderProject(dir, name, true)
	}
}
