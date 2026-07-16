# Daemon Bearer Authentication

## Scope

agent-compose optionally authenticates HTTP and HTTPS control-plane requests
with one daemon-wide shared Bearer token. The feature is intended to protect a
daemon endpoint from unauthenticated control-plane operations without adding
users, sessions, JWTs, refresh tokens, or role-based authorization.

The token grants full control-plane access. It is not an identity or a
fine-grained permission boundary.

## Daemon configuration

`AGENT_COMPOSE_AUTH_TOKEN` controls the feature:

- empty or unset: authentication is disabled;
- non-empty: protected HTTP(S) requests require
  `Authorization: Bearer <token>`;
- trusted Unix socket requests remain credential-free.

There is deliberately no separate enable flag or token-file option. This avoids
contradictory states such as authentication being enabled without a token. The
daemon loads the token once during startup and never writes or logs it.

Presented and configured tokens are hashed with SHA-256 before constant-time
comparison. Authentication failures return HTTP 401 with
`WWW-Authenticate: Bearer realm="agent-compose"` and `Cache-Control: no-store`.

## Route policy

Authentication is default-on for network routes when the daemon token is set.
Only boundaries that already have another credential or operational trust
model are explicitly exempt.

| Boundary | Daemon token | Reason |
| --- | --- | --- |
| Connect control-plane services | Required | Full daemon operations |
| `/api/version` and other daemon APIs | Required | Used to verify CLI login and prevent unauthenticated discovery |
| Workspace file APIs | Required | Read/write project data |
| Health Connect service | Exempt | Orchestrator readiness and liveness |
| Runtime LLM facade | Exempt | Uses a per-sandbox facade token |
| Jupyter proxy | Exempt | Uses the Jupyter/UI access boundary |
| `POST /api/webhooks/:topic` | Exempt | Uses each webhook source token |
| Webhook source management and event APIs | Required | Administrative control-plane operations |

Unknown future routes are protected by default. An exemption must be narrow,
documented, and backed by another authentication or trust boundary.

## CLI login and credential storage

The CLI verifies and saves credentials with:

```bash
agent-compose --host https://compose.example.com auth login --token '<token>'
```

Login sends the supplied token to the protected `/api/version` endpoint. The
credential is persisted only after a successful response. `auth logout`
removes one site and `auth list` prints site names without tokens.

The default store is the platform user configuration directory at
`agent-compose/config.yml`. On standard Linux systems this is
`~/.config/agent-compose/config.yml`. `AGENT_COMPOSE_CONFIG` can override the
path for controlled environments and tests.

```yaml
version: 1
hosts:
  https://compose.example.com:
    token: secret
```

Writes use an atomic same-directory rename and mode `0600`. Tokens are keyed by
the normalized daemon base URL. HTTP(S) client construction automatically loads
the matching token and injects it into ordinary, streaming, and attach
requests. Unix socket client construction does not read or send saved tokens.

## Transport and proxy considerations

Bearer authentication does not encrypt traffic. Plain HTTP remains supported
for loopback container mappings and controlled private deployments, but an
observer can capture and replay the token. Cross-machine deployments should
use HTTPS, an SSH tunnel, a VPN, or equivalent transport protection.

The middleware authenticates the daemon control plane, not a specific client
program. A UI server or reverse proxy that calls protected daemon APIs must
inject the same Authorization header before daemon authentication is enabled.
Authorization headers and configuration contents must be redacted from logs,
diagnostics, and support bundles.
