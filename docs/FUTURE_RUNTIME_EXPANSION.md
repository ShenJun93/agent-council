# Future Runtime Expansion — Subscription + API

## Status

**Blocked: the Phase H measurable-value gate is not satisfied.**

The governed Phase H technical value-validation pilot completed on September 3, 2026. The frozen 10-case technical pilot executed successfully with 120 evaluator calls, but its pre-registered outcome was **FAIL** because Council mean delta versus the strongest frozen single-agent baseline was `-1.38` (non-positive). There were zero materially-worse cases. Product-domain value remains untested because no complete frozen product A-F candidate corpus was available.

Source-of-truth audit references: issue [`#56`](https://github.com/ShenJun93/agent-council/issues/56) and workflow run [`33747506790`](https://github.com/ShenJun93/agent-council/actions/runs/33747506790).

This document records an architectural extension point so the public core does not accidentally make future API support difficult. It does **not** authorize API/BYOK implementation, and the completed Phase H result must not be reinterpreted by post-hoc threshold changes.

---

## 1. Core rule

Council Core calls only:

```go
type AgentRuntime interface {
    Run(ctx context.Context, req AgentRequest) (AgentResponse, error)
}
```

Council Core must not care whether execution uses:

- local subprocess
- authenticated CLI session
- direct HTTP API
- future structured runtime protocol

---

## 2. Runtime families

### v0

```text
ClaudeCLIRuntime
  transport: subprocess
  auth: Claude subscription OAuth
  billing: subscription

CodexCLIRuntime
  transport: subprocess
  auth: ChatGPT login
  billing: subscription
```

### Possible post-v0

```text
AnthropicAPIRuntime
  transport: HTTPS
  auth: API credential
  billing: metered API

OpenAIAPIRuntime
  transport: HTTPS
  auth: API credential
  billing: metered API
```

API runtimes are distinct implementations. Do not mutate CLI runtimes into API mode by injecting API keys.

---

## 3. Billing modes

Future config shape:

```yaml
billing:
  mode: subscription_only
  fail_closed: true
  allow_metered_fallback: false
```

Allowed future values:

```text
subscription_only
api_allowed
mixed
```

### subscription_only

Only approved subscription runtimes execute.

API credentials/runtimes are rejected.

### api_allowed

API runtimes may execute when explicitly assigned.

This does not mean subscription failures can fall back to API.

### mixed

A single Council run may contain explicitly configured subscription and API participants.

Example:

```yaml
participants:
  researcher_a:
    runtime: claude_cli

  researcher_b:
    runtime: openai_api
    model: provider-model-id
```

Every assignment is explicit.

---

## 4. Absolute prohibition: silent fallback

Forbidden:

```text
subscription runtime
→ quota exhausted
→ API key detected
→ continue using metered API
```

Required:

```text
subscription runtime
→ quota exhausted
→ classify quota_exhausted
→ mark arm/run incomplete according to protocol
→ operator decides whether to start a new explicitly API-backed run
```

This protects:

- billing expectations
- benchmark integrity
- reproducibility
- auditability

---

## 5. Why API may be valuable later

API mode is not inherently “better” than subscription mode.

Potential advantages for controlled experiments:

- more explicit model selection
- model snapshot/version pinning when a provider offers it
- easier request metadata capture
- higher-throughput execution where provider limits permit
- token/usage accounting

Potential commercial advantages:

- enterprise BYOK
- tenant-specific provider policy
- cost ceilings
- usage accounting
- audit logs
- hosted execution

---

## 6. Open-core placement

### Public core

Must retain:

- `AgentRuntime` contract
- protocol
- visibility firewall
- independence policy
- artifact/eval contracts
- subscription runtimes used by public v0

### Pro candidates

May contain:

- direct API runtime implementations
- BYOK
- secret/credential management
- tenant isolation
- usage/cost governance
- enterprise policy
- hosted operations

The public core must never depend on those Pro features.

---

## 7. Gate

No API/BYOK implementation before:

1. Phase C proves filesystem blindness.
2. Phase H pilot shows Council has measurable value.
3. A concrete pain exists that subscription-only execution cannot address economically or experimentally.

The September 3, 2026 Phase H technical pilot did **not** satisfy item 2. Therefore API/BYOK implementation remains blocked under the existing gate. Any change to that gate requires a separate repository-governance decision; this status update does not weaken, replace, rerun, or reinterpret the completed Phase H result.
