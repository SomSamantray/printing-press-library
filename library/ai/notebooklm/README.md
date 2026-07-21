# notebooklm-pp-cli

Go CLI for [Gemini Notebook](https://notebooklm.google.com/) (formerly NotebookLM) using Google's undocumented `batchexecute` RPC surface.

> **Unofficial.** Uses your Google Chrome session cookies. Not affiliated with Google. Personal use only.

## Features

### Parity with notebooklm-py (in progress)

| Area | Commands | Status |
|------|----------|--------|
| Auth | `auth login --chrome`, `auth status --json` | v0.1 |
| Notebooks | `notebook list --json` | v0.1 |
| Doctor | `doctor --json` | v0.1 |
| Sources, chat, studio, share, MCP | — | planned (see `docs/plans/2026-07-21-001-feat-notebooklm-cli-plan.md`) |

### Unique advantages over notebooklm-py

- **Native Go binary** — single static binary, no Python runtime or venv
- **Printing Press conventions** — `--json` on every command, `doctor --json`, Steinberger agent hints
- **Discovery artifacts** — Chrome DevTools UI→RPC map committed under `docs/discovery/notebooklm/`
- **Hand-authored RPC layer** — typed Go client mirroring `google-trends-pp-cli` batchexecute patterns
- **SQLite cache + FTS** (planned) — offline search across notebooks, sources, and chat history
- **MCP server** (planned) — intent-shaped tools matching CLI, stdio transport

## Install

```bash
cd library/ai/notebooklm
go build -o notebooklm-pp-cli ./cmd/notebooklm-pp-cli/
```

## Auth

```bash
pip install pycookiecheat   # or use --cookies-file
notebooklm-pp-cli auth login --chrome
notebooklm-pp-cli auth status --json
```

## Usage

```bash
notebooklm-pp-cli doctor --json
notebooklm-pp-cli notebook list --json
```

## Discovery

Browser exploration and RPC catalog:

- `docs/discovery/notebooklm/2026-07-21-ui-feature-map.md`
- `docs/discovery/notebooklm/2026-07-21-ui-rpc-map.md`
- `docs/discovery/notebooklm/2026-07-21-traffic-analysis.json`

## Development

```bash
go test ./...
go build ./cmd/notebooklm-pp-cli/
```

## License

Apache-2.0
