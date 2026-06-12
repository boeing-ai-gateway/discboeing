# Provider Support in agent-go

This document catalogs all providers from [models.dev](https://models.dev), maps them to Go implementation status, and prioritizes remaining work.

## Implementation Status Overview

| Go Implementation | Provider Groups Covered | Providers Covered |
|---|---|---|
| `providers/anthropic` | Anthropic-compatible APIs | 7 providers |
| `providers/openai` | OpenAI native and OpenAI-compatible APIs | 2 + 67 providers |
| **Not yet implemented** | Distinct provider protocols | ~20 providers |

The two existing Go implementations cover **76 of 96 providers** (79%) by routing through:
- Anthropic Messages API (`/v1/messages`) — for anthropic-protocol providers
- OpenAI Responses API (`/v1/responses`, via HTTP SSE or pooled WebSocket mode) — for OpenAI native providers
- OpenAI Chat Completions API (`/v1/chat/completions`) — for openai-compatible providers

The remaining 20 providers use distinct wire protocols that require dedicated implementations.

---

## Priority 1 — Critical (Implement First)

These providers have large enterprise/consumer adoption and distinct APIs that cannot be served by the existing openai-compatible path.

| Provider ID | Status | Notes |
|---|---|---|
| `google` | Not implemented | Gemini models; uses Google AI Studio API — distinct auth (API key or service account) and REST format |
| `google-vertex` | Not implemented | Gemini via GCP Vertex AI; OAuth2/service account, regional endpoints |
| `amazon-bedrock` | Not implemented | Multi-model AWS service; AWS SigV4 auth, distinct converse/invoke APIs |
| `azure` | Not implemented | OpenAI models via Azure; Azure AD / API key auth, versioned deployment URLs |
| `azure-cognitive-services` | Not implemented | Shares implementation with `azure` |

---

## Priority 2 — High (Widely Used, Distinct APIs)

Popular providers that are not openai-compatible or have important behavioural differences.

| Provider ID | Status | Notes |
|---|---|---|
| `openrouter` | Not implemented | Meta-provider over many models; OpenAI-compatible API but with `HTTP-Referer` / `X-Title` headers and model routing semantics |
| `groq` | Not implemented | OpenAI-compatible endpoint with tool call and reasoning quirks |
| `mistral` | Not implemented | Own REST API; function calling format differs from OpenAI |
| `cohere` | Not implemented | Distinct Chat API (`/v2/chat`); connector / RAG features |
| `xai` | Not implemented | Grok models; largely OpenAI-compatible with Grok-specific fields |

---

## Priority 3 — Medium (Notable, Addressable)

Providers with meaningful user bases or useful routing/gateway functionality.

| Provider ID | Status | Notes |
|---|---|---|
| `cloudflare-ai-gateway` | Not implemented | Acts as a caching/routing proxy in front of other providers; auth via Cloudflare account ID + gateway ID |
| `vercel` | Not implemented | Vercel's unified AI Gateway; token-based, routes to multiple backends |
| `togetherai` | Not implemented | Open-weight model hosting; mostly OpenAI-compatible |
| `perplexity` | Not implemented | Search-augmented completions; OpenAI-compatible endpoint with `search_domain_filter` etc. |
| `deepinfra` | Not implemented | Open model hosting; OpenAI-compatible |
| `cerebras` | Not implemented | Wafer-scale inference; OpenAI-compatible |
| `google-vertex-anthropic` | Not implemented | Claude models served through GCP Vertex AI; shares Vertex auth but uses Anthropic message format |

---

## Priority 4 — Lower (Niche / Specialized)

Providers with narrower audiences, third-party SDKs, or less urgency.

| Provider ID | Status | Notes |
|---|---|---|
| `v0` | Not implemented | Vercel v0 code-gen service; Vercel token auth |
| `gitlab` | Not implemented | GitLab Duo AI; enterprise GitLab auth |
| `sap-ai-core` | Not implemented | SAP Business Technology Platform AI; complex service key auth |
| `venice` | Not implemented | Privacy-focused inference; OpenAI-compatible but distinct auth |

---

## Already Supported Providers

### Via `providers/anthropic` (7 providers)

These all speak the Anthropic Messages API (`POST /v1/messages`). The `anthropic` provider is registered and handles routing via a configurable base URL.

| Provider ID | Notes |
|---|---|
| `anthropic` | Primary Anthropic API |
| `kimi-for-coding` | Moonshot Kimi, Anthropic-compatible endpoint |
| `minimax` | MiniMax international, Anthropic-compatible |
| `minimax-cn` | MiniMax China region |
| `minimax-cn-coding-plan` | MiniMax China coding plan tier |
| `minimax-coding-plan` | MiniMax coding plan tier |
| `zenmux` | Zenmux proxy, Anthropic-compatible |

### Via `providers/openai` — OpenAI native (2 providers)

These use the OpenAI Responses API (`POST /v1/responses`). The Go implementation defaults to HTTP SSE (`https://api.openai.com/v1`) because it is more stable, and still supports pooled WebSocket mode when `use_websocket=true` is configured or the base URL uses a `ws://`/`wss://` scheme. In WebSocket mode it raises the per-message read limit so large response events do not fail at the transport layer, automatically replaces stale pooled sockets, and falls back to replaying full history when a `previous_response_id` cache entry is gone.

| Provider ID | Notes |
|---|---|
| `openai` | Primary OpenAI API |
| `vivgrid` | VivGrid, routes through OpenAI endpoint |

### Via `providers/openai` — OpenAI-compatible (67 providers)

All use the Chat Completions API (`POST /v1/chat/completions`) with a provider-specific base URL resolved from the models.dev metadata.

<details>
<summary>Show all 67 providers</summary>

302ai, abacus, aihubmix, alibaba, alibaba-cn, bailing, baseten, berget, chutes,
cloudferro-sherlock, cloudflare-workers-ai, cortecs, deepseek, evroc, fastrouter,
fireworks-ai, firmware, friendli, github-copilot, github-models, helicone, huggingface,
iflowcn, inception, inference, io-net, jiekou, kilo, kuae-cloud-coding-plan, llama,
lmstudio, lucidquery, meganova, moark, modelscope, moonshotai, moonshotai-cn, morph,
nano-gpt, nebius, nova, novita-ai, nvidia, ollama-cloud, opencode, ovhcloud, poe,
privatemode-ai, qihang-ai, qiniu-ai, requesty, scaleway, siliconflow, siliconflow-cn,
stackit, stepfun, submodel, synthetic, upstage, vultr, wandb, xiaomi, zai,
zai-coding-plan, zhipuai, zhipuai-coding-plan

</details>

---

## Provider Group → Go Implementation Mapping

| Provider Group | # Providers | Go Implementation |
|---|---|---|
| OpenAI-compatible Chat Completions | 67 | `providers/openai` (chat completions path) |
| Anthropic-compatible Messages | 7 | `providers/anthropic` |
| OpenAI Responses | 2 | `providers/openai` (responses API path) |
| Distinct provider protocols | 21 | **TODO** |

**Total: 23 provider groups → 2 implemented, 21 TODO**

---

## Implementation Notes

### OpenAI-Compatible Shortcuts

Several of the "not yet implemented" providers actually expose an OpenAI-compatible Chat Completions endpoint. The following could be added quickly by registering an alias in the `openai` provider with a custom base URL:

- `groq` — `https://api.groq.com/openai/v1`
- `xai` — `https://api.x.ai/v1`
- `deepinfra` — `https://api.deepinfra.com/v1/openai`
- `cerebras` — `https://api.cerebras.ai/v1`
- `togetherai` — `https://api.together.xyz/v1`
- `perplexity` — `https://api.perplexity.ai`
- `openrouter` — `https://openrouter.ai/api/v1`
- `venice` — `https://api.venice.ai/api/v1`

These require dedicated providers only if their extra features (search filters, routing headers, etc.) need to be surfaced.

### Hardest to Implement

- **Amazon Bedrock** — AWS SigV4 request signing, region-scoped endpoints, and a multi-model converse API that differs significantly from OpenAI Chat Completions.
- **Google Vertex AI** — GCP service account / Workload Identity auth, regional endpoints, and the Gemini API format.
- **SAP AI Core** — Complex service key auth (XSUAA OAuth) and SAP-specific API paths.

## Test-only e2e mock LLM provider

`agent-go` can be built with the explicit Go build tag `e2e_mock_llm` to register a deterministic provider for full-stack e2e tests without broad UI/API mocking:

```sh
go build -tags=e2e_mock_llm ./cmd/agent-api
DISCOBOT_MODEL=e2e-mock-llm/mock ./agent-api -server
```

The provider ID is `e2e-mock-llm` and its default model is `mock`. It is not compiled or registered in normal builds. The build-tagged provider embeds JSON fixtures from `agent-go/llm-responses/*.json`; tests can override the fixture directory at runtime with `DISCOBOT_E2E_MOCK_LLM_RESPONSES_DIR=/path/to/fixtures`.

Fixture files are merged in filename order. Each file maps input text to deterministic responses:

```json
{
  "responses": [
    {
      "name": "greeting",
      "match": { "exact": "hello" },
      "response": { "text": "Hello from the mock LLM." }
    },
    {
      "name": "run pwd",
      "match": { "contains": "where am I" },
      "response": {
        "toolCalls": [
          { "name": "Bash", "input": "{\"command\":\"pwd\"}" }
        ]
      }
    }
  ],
  "fallback": { "text": "Default e2e mock response." }
}
```

Matching uses the latest non-synthetic user text and supports `exact`, `contains`, and `regex` fields. If no fixture matches and no fallback is configured, the provider returns a clear error.
