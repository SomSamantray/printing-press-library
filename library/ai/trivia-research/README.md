# trivia-research-pp-cli

Deep research across 9 podcast transcript and blog archives. Search Gastropod, Freakonomics Radio, Planet Money, The Indicator, Twenty Thousand Hertz, 99% Invisible, Acquired, Business Breakdowns, and SatPost in parallel for deep topic coverage.

## Quick Start

```bash
# Search all 9 sources
trivia-research-pp-cli research "fermentation"

# Search specific sources
trivia-research-pp-cli research "economics" --sources planetmoney,indicator,freakonomics

# Structured JSON output for agents
trivia-research-pp-cli research "AI intelligence" --json

# Agent-friendly defaults
trivia-research-pp-cli research "supply chain" --agent

# Health check all sources
trivia-research-pp-cli doctor

# Pre-fetch episode archives to local store
trivia-research-pp-cli sync
```

## Install

```bash
go install ./cmd/trivia-research-pp-cli@latest
```

Or build from source:

```bash
git clone https://github.com/mvanhorn/printing-press-library
cd trivia-research-pp-cli
make build
```

## Commands

| Command | Description |
|---------|-------------|
| `research "topic"` | Search all 9 sources in parallel |
| `research --sources x,y` | Filter to specific sources |
| `research --ask "question?"` | Natural language → keyword extraction |
| `research --json/--csv/--compact` | Output format |
| `research --agent` | Agent-friendly defaults |
| `research --data-source auto/live/local` | Offline-first strategy |
| `research --select title,score,source` | Field filtering |
| `sync` | Pre-fetch episode archives |
| `doctor` | Health-check all sources |
| `version` | Version info |

## Sources

- 🥄 **Gastropod** — Food through science & history (gastropod.com)
- 📈 **Freakonomics Radio** — Hidden side of everything (freakonomics.com)
- 🌍 **Planet Money** — Economics explained (npr.org)
- 💡 **The Indicator** — Bite-sized economics (npr.org)
- 🎧 **Twenty Thousand Hertz** — Stories about sound (20k.org)
- 🏛️ **99% Invisible** — Design & architecture (99percentinvisible.org)
- 📰 **SatPost Blog** — Tech, business & memes (readtrung.com)
- 📡 **Acquired** — Company deep dives (acquired.fm)
- 🏦 **Business Breakdowns** — Business model analysis (joincolossus.com)

## Agent Usage

```bash
# JSON output with selected fields (token-efficient)
trivia-research-pp-cli research "economics" --agent --select title,url,score,source

# Compact markdown for synthesis
trivia-research-pp-cli research "AI" --emit compact
```

## Troubleshooting

- **Source unreachable:** Run `trivia-research-pp-cli doctor` to check connectivity
- **No results:** Try a broader keyword or check `--sources` filter
- **Slow search:** Reduce `--max-episodes` (default 50)
- **Network errors:** Some sources may be temporarily down; retry

## License

Apache-2.0
