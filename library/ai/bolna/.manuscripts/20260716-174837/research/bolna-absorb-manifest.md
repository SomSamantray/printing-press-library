# Bolna CLI Absorb Manifest

## Shipping scope

- 78 generated API endpoints across the documented and dashboard-observed resource interfaces.
- All documented API reference groups represented in the OpenAPI source, including subaccounts and violations.
- Stable dashboard-discovered read routes represented where Chrome exposed them.
- Main-account and subaccount CRUD, usage, concurrency, and main-organization patch commands.
- Agent-friendly JSON/compact output, stdin request bodies, dry-run, profiles, sync/search, and MCP artifacts.
- Cross-account `analytics report` with multiple environment-backed bearer scopes, date/agent/provider/status filters, metric rollups, trends, insights, duplicate-page protection, and bounded pagination.

## Deliberate exclusions

- Browser-cookie or resident-browser runtime: discovery only; the CLI uses replayable HTTP with bearer API keys.
- Dashboard-only UI actions without an observed/documented API contract, such as visual graph editing internals.
- Deprecated v1 agent paths: v2 paths are the shipping API surface.

## Auth gate

- Required env var: `BOLNA_BEARER_AUTH`.
- Header: `Authorization: Bearer <token>`.
- Main API key: Dashboard > Developers.
- Subaccount API key: Bolna docs state these start with `sa-`.

## Live validation note

- Main and subaccount credentials were tested only through ephemeral environment variables and only against read-only endpoints. Credential values are intentionally not stored in this manifest or any generated artifact.
- June–July 2026 validation was run against both newly supplied subaccount scopes. One selected agent produced 5,000 capped executions and derived insights; the other scope authenticated but its selected agent had no records in the window. An all-agent probe timed out on an upstream execution route and was safely terminated.
