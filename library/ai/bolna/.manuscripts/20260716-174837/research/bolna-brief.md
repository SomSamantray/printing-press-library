# Bolna CLI Brief

## API Identity

- Domain: `https://api.bolna.ai`
- Product: Bolna Voice AI, a multilingual conversational calling platform.
- Users: teams automating outbound/inbound calls, campaigns, support, lead qualification, scheduling, and enterprise workspaces.
- Auth: every endpoint requires `Authorization: Bearer <API key>`. Main-account keys are created in Dashboard > Developers; keys created for a subaccount start with `sa-`.

## Research Evidence

- Official API index: `https://www.bolna.ai/docs/llms.txt`.
- API groups: agents, calls, executions/raw logs, batches, inbound, knowledge bases, phone numbers, providers, voices, SIP trunks, dispositions, user/model, violations, and subaccounts.
- Authenticated Chrome dashboard routes observed: `/v2/agent/list`, `/agent/list`, `/prompt-library/modules`, `/api/v1/voice-config/languages`, `/user/model/all`, `/user/webhook/allowed-states`, `/ambient-sounds`, `/assistants/get_gpt_assistants`, `/get_current_price`, `/sub-accounts/all`, and `/sub-accounts/concurrency`.
- Dashboard areas: Agent Studio, Graph Agent, Call History, My Numbers, SIP Trunks, Knowledge Base, Batches, Reports, Analytics, Developers, Providers, Account, Compliance, Invoices, Violations, and Sub Accounts.

## Live Read-Only Validation

- Main-account bearer credential authenticated successfully: account metadata, 40 agents, 114 subaccounts, organization concurrency, 6 phone numbers, 10 knowledge bases, providers, and report jobs were readable.
- Subaccount bearer credential authenticated successfully for its scoped data: 8 agents and 14 phone numbers were readable.
- The subaccount credential does not expose organization-only routes: `/user/me` and `/sub-accounts/all` returned 404 in the subaccount scope. The CLI should surface this as scope behavior, not as a malformed-key diagnosis.
- No outbound calls, creates, updates, deletes, purchases, schedules, or other mutating operations were run during validation.

## Top Workflows

1. Inspect account, agents, supported languages/providers/voices, then create or update an agent.
2. Place an outbound call, poll execution status, retrieve transcript/recording/extracted data, and fetch raw logs.
3. Upload a CSV batch, schedule it, monitor executions, and stop queued work.
4. Provision inbound routing, phone numbers, provider credentials, or a SIP trunk.
5. Manage an organization’s subaccounts, concurrency envelope, usage, and billing views.
6. Compare call-history executions across multiple bearer-key scopes, filter by UTC date interval, agent, provider, and status, then derive rollups, trends, and evidence-based insights.

## Product Thesis

Install this CLI to give agents one machine-readable command tree for the complete Bolna API, including organization-admin subaccount controls, local sync/search, dry runs, stdin JSON, and MCP output.

## Data Layer

Primary entities are agents, executions, batches, phone numbers, knowledge bases, providers, voices, SIP trunks, dispositions, subaccounts, usage, and account state. Bolna documents `page_number`/`page_size` pagination with a maximum page size of 50; execution polling should prefer webhooks and wait for terminal statuses.

## Explicit Token Requirement

Set `BOLNA_BEARER_AUTH` to a Bolna API key before live commands. Use a main-account key for main-account operations. For subaccount-isolated operations, use that subaccount’s `sa-...` key or a separate profile/environment containing it. Never commit the key. Browser cookies are discovery-only.

## Differentiated Analytics Feature

The generated CLI now includes `analytics report` (aliases: `compare`, `deep-dive`). It accepts repeatable `--source LABEL=ENV_VAR` inputs, fetches each account independently, filters execution history by inclusive `--from`/`--to` dates, and produces account rollups, agent rollups, daily/weekly/status/provider trends, success/answer/duration/cost/latency metrics, and deterministic insights. Credentials remain environment-only. Pagination has duplicate-page detection, date-aware stopping, and a configurable `--max-pages` safety bound.

Live June–July validation used two subaccount bearer scopes. The second scope returned 5,000 records for a selected agent before the deliberate `--max-pages=100` cap and produced success, failure, duration, cost, and trend insights. The first scope authenticated and exposed its agent inventory; its selected agent returned no executions in the requested window, while a broader all-agent probe encountered an upstream timeout on one execution route and was stopped safely.
