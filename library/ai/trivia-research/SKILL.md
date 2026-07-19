---
name: trivia-research
version: "1.0.0"
description: "Deep research across podcast transcripts and blog archives."
user-invocable: true
---

## Prerequisites: Install the CLI

This skill drives the `trivia-research-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install trivia-research --cli-only
   ```
2. Verify: `trivia-research-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/trivia-research/cmd/trivia-research-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Deep research across 9 podcast transcript and blog archives: Gastropod, Freakonomics Radio, Planet Money, The Indicator, 20K Hz, 99% Invisible, SatPost, Acquired, and Business Breakdowns.

## When to Use This CLI

Use this CLI to search podcast transcripts and blog posts programmatically across multiple sources in parallel, with keyword scoring and transcript deep-fetch for high-confidence matches.

## Output Contract

**BADGE:** The engine emits:
```
🔬 DeepResearch v{VERSION} · synced {YYYY-MM-DD}
```
Pass through verbatim.

**LAW 1:** No Sources block at end.
**LAW 2:** No invented title line.
**LAW 3:** No em-dashes.
**LAW 4:** No ## section headers in body.
**LAW 5:** Engine footer pass-through.
**LAW 6:** No raw evidence clusters.
**LAW 7:** Cite sources inline (≥3 source names).
**LAW 8:** Host-adaptive URL formatting.

## How to Invoke

```bash
trivia-research-pp-cli research "economics" --agent
```

For structured output:
```bash
trivia-research-pp-cli research "AI intelligence" --json --select title,url,score
```

For agent synthesis:
```bash
trivia-research-pp-cli research "topic" --emit compact
```
