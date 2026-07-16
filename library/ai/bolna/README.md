# Bolna Pp Cli CLI

**Operate Bolna Voice AI agents, calls, campaigns, telephony, and enterprise subaccounts from one machine-readable CLI.**

Turn the documented Bolna API and stable dashboard-discovered routes into agent-friendly commands with JSON, compact output, dry runs, stdin bodies, local sync/search, and MCP.

## Install

The recommended path installs both the `bolna-pp-cli` binary and the `pp-bolna` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install bolna-pp-cli
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install bolna-pp-cli --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install bolna-pp-cli --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install bolna-pp-cli --agent claude-code
npx -y @mvanhorn/printing-press-library install bolna-pp-cli --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/bolna-pp-cli-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install bolna-pp-cli --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-bolna --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-bolna --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install bolna-pp-cli --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/bolna-pp-cli-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `BOLNA_PP_CLI_BEARER_AUTH` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "bolna-pp-cli": {
      "command": "bolna-pp-mcp",
      "env": {
        "BOLNA_PP_CLI_BATCH_ID": "<batch_id>",
        "BOLNA_PP_CLI_BEARER_AUTH": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

```bash
# List agents.
bolna-pp-cli agent list --json

# List organization subaccounts.
bolna-pp-cli sub-accounts list --json

# Read current call pricing.
bolna-pp-cli get-current-price --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Enterprise operations
- **`sub-accounts`** — Manage main organization concurrency, subaccount CRUD, usage, and billing from one agent-facing tree.

  _Use for multi-customer or multi-team Bolna organizations._

  ```bash
  bolna-pp-cli sub-accounts list --json
  ```

### Voice operations
- **`workflow`** — Combine agent configuration, outbound calls, execution polling, transcript retrieval, and raw-log inspection.

  _Use for reliable outbound calling automation._

  ```bash
  bolna-pp-cli call --stdin --json
  ```

## Recipes

### List subaccounts

```bash
bolna-pp-cli sub-accounts list --json
```

Read the organization subaccount inventory.

### Inspect usage

```bash
bolna-pp-cli sub-accounts all-usage --json
```

Read organization-wide subaccount usage.

## Usage

Run `bolna-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `BOLNA_PP_CLI_CONFIG_DIR`, `BOLNA_PP_CLI_DATA_DIR`, `BOLNA_PP_CLI_STATE_DIR`, or `BOLNA_PP_CLI_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `BOLNA_PP_CLI_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export BOLNA_PP_CLI_HOME=/srv/bolna-pp-cli
bolna-pp-cli doctor
```

Under `BOLNA_PP_CLI_HOME=/srv/bolna-pp-cli`, the four dirs resolve to `/srv/bolna-pp-cli/config`, `/srv/bolna-pp-cli/data`, `/srv/bolna-pp-cli/state`, and `/srv/bolna-pp-cli/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "bolna-pp-cli": {
      "command": "bolna-pp-mcp",
      "env": {
        "BOLNA_PP_CLI_HOME": "/srv/bolna-pp-cli"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `BOLNA_PP_CLI_DATA_DIR` overrides an explicit `--home` for that kind. Use `BOLNA_PP_CLI_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `BOLNA_PP_CLI_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `bolna-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### agent

Manage agent

- **`bolna-pp-cli agent create`** - Create a voice AI agent
- **`bolna-pp-cli agent dashboard-list`** - Dashboard agent list with pagination
- **`bolna-pp-cli agent delete`** - Delete an agent and related data
- **`bolna-pp-cli agent get`** - Get agent details
- **`bolna-pp-cli agent legacy-dashboard-list`** - Legacy dashboard agent list
- **`bolna-pp-cli agent list`** - List all voice AI agents
- **`bolna-pp-cli agent patch`** - Partially update an agent
- **`bolna-pp-cli agent update`** - Replace an agent configuration

### ambient-sounds

Manage ambient sounds

- **`bolna-pp-cli ambient-sounds`** - List available ambient sounds

### assistants

Manage assistants

- **`bolna-pp-cli assistants`** - List GPT assistants available to the account

### batches

Manage batches

- **`bolna-pp-cli batches batch-create`** - Create a CSV batch
- **`bolna-pp-cli batches batch-delete`** - Delete a batch
- **`bolna-pp-cli batches batch-get`** - Get batch details

### bolna-analytics

Manage bolna analytics

- **`bolna-pp-cli bolna-analytics`** - List analytics dashboards

### analytics

Cross-account call-history analytics

- **`bolna-pp-cli analytics report`** - Compare accounts and agents over a date range

Use one environment variable per bearer key. Values never enter the report:

```bash
export BOLNA_SUB1_KEY='your-subaccount-key'
export BOLNA_SUB2_KEY='your-subaccount-key'
bolna-pp-cli analytics report \
  --source account-a=BOLNA_SUB1_KEY \
  --source account-b=BOLNA_SUB2_KEY \
  --from 2026-06-01 --to 2026-07-16 \
  --metric all --group-by day --agent
```

The report includes account and agent rollups, daily/weekly/status/provider trends, duration, cost, latency, answer and success metrics when present, plus evidence-based comparison insights. Use `--agent-id`, `--agent-name`, `--provider`, `--status`, `--max-pages`, and repeatable `--metric` flags to narrow or expand the analysis.

### call

Manage call

- **`echo '{\"agent_id\":\"<agent-id>\",\"recipient_phone_number\":\"+15551234567\"}' | bolna-pp-cli call --stdin --json`** - Make an outbound voice AI call from a JSON request body

### compliance

Manage compliance

- **`bolna-pp-cli compliance`** - List phone-number compliance records

### dispositions

Manage dispositions

- **`bolna-pp-cli dispositions bulk-create`** - Bulk create and link dispositions
- **`bolna-pp-cli dispositions create`** - Create and link a disposition
- **`bolna-pp-cli dispositions delete`** - Delete a disposition
- **`bolna-pp-cli dispositions get`** - Get a disposition
- **`bolna-pp-cli dispositions list`** - List dispositions
- **`bolna-pp-cli dispositions update`** - Update a disposition

### executions

Manage executions

- **`bolna-pp-cli executions <execution_id>`** - Get execution details

### get-current-price

Manage get current price

- **`bolna-pp-cli get-current-price`** - Calculate current call price for a provider configuration

### inbound

Manage inbound

- **`bolna-pp-cli inbound setup`** - Link an agent to a phone number for inbound calls
- **`bolna-pp-cli inbound unlink`** - Unlink an inbound agent

### invoices

Manage invoices

- **`bolna-pp-cli invoices`** - List account invoices

### keys

Manage keys

- **`bolna-pp-cli keys`** - List API keys visible to the account

### knowledgebase

Manage knowledgebase

- **`bolna-pp-cli knowledgebase create`** - Create a knowledge base from PDF or URL
- **`bolna-pp-cli knowledgebase delete`** - Delete a knowledge base
- **`bolna-pp-cli knowledgebase get`** - Get a knowledge base
- **`bolna-pp-cli knowledgebase list`** - List knowledge bases

### phone-numbers

Manage phone numbers

- **`bolna-pp-cli phone-numbers buy`** - Buy a phone number
- **`bolna-pp-cli phone-numbers delete`** - Delete a purchased phone number
- **`bolna-pp-cli phone-numbers list`** - List phone numbers
- **`bolna-pp-cli phone-numbers search`** - Search available phone numbers

### prompt-library

Manage prompt library

- **`bolna-pp-cli prompt-library`** - List published prompt modules

### providers

Manage providers

- **`bolna-pp-cli providers add`** - Add a provider credential
- **`bolna-pp-cli providers list`** - List configured providers
- **`bolna-pp-cli providers remove`** - Remove a provider

### reports

Manage reports

- **`bolna-pp-cli reports`** - List report jobs

### sip-trunks

Manage sip trunks

- **`bolna-pp-cli sip-trunks add-number`** - Add a DID to a SIP trunk
- **`bolna-pp-cli sip-trunks create`** - Create a SIP trunk
- **`bolna-pp-cli sip-trunks delete`** - Delete a SIP trunk
- **`bolna-pp-cli sip-trunks get`** - Get a SIP trunk
- **`bolna-pp-cli sip-trunks list`** - List SIP trunks
- **`bolna-pp-cli sip-trunks numbers`** - List phone numbers on a SIP trunk
- **`bolna-pp-cli sip-trunks remove-number`** - Remove a number from a SIP trunk
- **`bolna-pp-cli sip-trunks update`** - Update a SIP trunk

### sub-accounts

Manage sub accounts

- **`bolna-pp-cli sub-accounts all-usage`** - Get usage for all subaccounts
- **`bolna-pp-cli sub-accounts concurrency`** - Get organization concurrency envelope
- **`bolna-pp-cli sub-accounts create`** - Create a subaccount
- **`bolna-pp-cli sub-accounts delete`** - Delete a subaccount and its data
- **`bolna-pp-cli sub-accounts list`** - List all subaccounts
- **`bolna-pp-cli sub-accounts main-patch`** - Update main organization concurrency settings
- **`bolna-pp-cli sub-accounts patch`** - Update subaccount name or concurrency

### user

Manage user

- **`bolna-pp-cli user add-custom-model`** - Add a custom LLM model
- **`bolna-pp-cli user dashboard-bootstrap`** - Get authenticated dashboard bootstrap context
- **`bolna-pp-cli user get`** - Get account information
- **`bolna-pp-cli user list-models`** - List custom and available models
- **`bolna-pp-cli user webhook-allowed-states`** - List webhook allowed states

### violations

Manage violations

- **`bolna-pp-cli violations list`** - List account violations
- **`bolna-pp-cli violations submit`** - Submit a violation report

### voice-config

Manage voice config

- **`bolna-pp-cli voice-config language-list`** - List supported languages
- **`bolna-pp-cli voice-config voice-list`** - List voices for a TTS provider and model
- **`bolna-pp-cli voice-config voice-providers`** - List TTS providers and models


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`bolna-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`bolna-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`bolna-pp-cli learnings list`** - Inspect taught rows
- **`bolna-pp-cli learnings forget <query>`** - Undo a teach
- **`bolna-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`bolna-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`bolna-pp-cli teach-pattern`** - Install a query/resource template up front
- **`bolna-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `BOLNA_PP_CLI_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `bolna-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
bolna-pp-cli agent list

# JSON for scripting and agents
bolna-pp-cli agent list --json

# Filter to specific fields
bolna-pp-cli agent list --json --select id,name,status

# Dry run — show the request without sending
bolna-pp-cli agent list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
bolna-pp-cli agent list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Runtime Endpoint

This CLI resolves endpoint placeholders at runtime, so one installed binary can target different tenants or API versions without regeneration.

Endpoint environment variables:
- `BOLNA_PP_CLI_BATCH_ID` resolves `{batch_id}`

Base URL: `https://api.bolna.ai`

## Health Check

```bash
bolna-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `bolna-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/bolna-pp-cli/config.toml`; `--home`, `BOLNA_PP_CLI_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `BOLNA_PP_CLI_BATCH_ID` | endpoint | Yes |  |
| `BOLNA_PP_CLI_BEARER_AUTH` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `bolna-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `bolna-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $BOLNA_PP_CLI_BEARER_AUTH`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

### API-specific
- **401 Unauthorized** — Set BOLNA_BEARER_AUTH to a Bolna API key. Use a main-account key for main-account actions or an sa- subaccount key for isolated subaccount actions.
