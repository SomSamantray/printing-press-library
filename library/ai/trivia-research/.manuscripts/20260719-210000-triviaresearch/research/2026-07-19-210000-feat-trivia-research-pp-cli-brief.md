# trivia-research-pp-cli Brief

## API Identity
Multi-source podcast transcript and blog archive search CLI. Searches 9 sources in parallel for deep topic research.

## Sources
Gastropod (food), Freakonomics Radio (economics), Planet Money (economics), The Indicator, 20K Hz (sound), 99% Invisible (design), SatPost (blog), Acquired (company deep dives), Business Breakdowns (Colossus).

## Architecture
Go CLI with parallel goroutine-based search, keyword scoring, transcript deep-fetch, and multiple output formats (JSON, CSV, table, compact markdown, plain).

## Build Priorities
1. Parallel search across 9 sources
2. Keyword scoring with transcript enrichment
3. Multiple output formats
4. MCP server for agent consumption
