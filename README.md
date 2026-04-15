# claude-watch

Real-time monitoring for [Claude Code](https://claude.ai/code) API usage. Shows request counts, token consumption, and active sessions — directly in your terminal.

```
╭──────────────────────────────────┬────────┬─────────┬─────────┬────────────╮
│ PROJECT                          │    REQ │   INPUT │  OUTPUT │ STATUS     │
├──────────────────────────────────┼────────┼─────────┼─────────┼────────────┤
│ my-app                           │    242 │  276.7K │  186.2K │ ● active   │
│ old-session                      │      1 │     512 │      12 │ ○ closed   │
├──────────────────────────────────┼────────┼─────────┼─────────┼────────────┤
│ my-app                           │    243 │  277.2K │  186.2K │ cache 97%  │
╰──────────────────────────────────┴────────┴─────────┴─────────┴────────────╯
```

## Features

- **Per-project and per-session** breakdown of API requests
- **Token usage**: input, output, cache writes, cache reads
- **Cache hit rate** — see how effectively prompt caching is working
- **Active session detection** — knows which Claude Code instances are running (via PID)
- **Watch mode** (`-w`) — live auto-refresh with flicker-free updates
- **All projects** (`-a`) — see usage across all your Claude Code projects

## Install

### From source

```bash
go install github.com/yasaricli/claude-watch/cmd/claude-watch@latest
```

### Build manually

```bash
git clone https://github.com/yasaricli/claude-watch.git
cd claude-watch
go build -o claude-watch ./cmd/claude-watch
```

### Homebrew

```bash
# Coming soon
```

## Usage

```bash
# Show current project (run from a directory where you've used Claude Code)
claude-watch

# Show all projects
claude-watch -a

# Watch mode — auto-refresh on file changes
claude-watch -w

# Watch all projects
claude-watch -w -a

# Specify a project path
claude-watch -p /path/to/my-project

# Custom polling interval (default: 2s)
claude-watch -w -i 5s
```

## How it works

`claude-watch` reads session data that Claude Code stores locally in `~/.claude/`:

- **`~/.claude/projects/<slug>/`** — JSONL files containing per-session API usage (tokens, model, cost)
- **`~/.claude/sessions/`** — JSON files mapping session IDs to running process PIDs

No API keys needed. No network requests. Everything is read from local files.

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-w` | false | Watch mode — auto-refresh on file changes |
| `-a` | false | Show all projects |
| `-p` | cwd | Path to project directory |
| `-i` | 2s | Polling interval for watch mode |

## License

MIT
