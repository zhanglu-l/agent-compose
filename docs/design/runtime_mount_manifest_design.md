# Runtime Mount Manifest

Chinese version: [../zh-CN/design/runtime_mount_manifest_design.md](../zh-CN/design/runtime_mount_manifest_design.md)

This document describes the current runtime mount manifest behavior in code.
Before starting or resuming runtime, agent-compose generates a driver-specific
manifest from one logical runtime mount list. Docker applies that list as
fine-grained binds. BoxLite and Microsandbox use a directory-only manifest that
mounts the whole session at `/data`, then guest bootstrap exposes compatible
paths such as `/workspace` and declared home entries under `/root`.

## Design Goals

Tools inside runtime should continue to use image-default directory semantics:

- Workspace lives at `/workspace`.
- `$HOME` uses the image default, currently `/root`.
- agent-compose internal exchange directories live at `/data/state`,
  `/data/runtime`, and `/data/logs`.

Host-side session state still lives under `<session>`. Docker does not expose
host control-plane state such as `context`, `vm`, `proxy`, and `metadata.json`.
Directory-only runtimes expose `<session>` at `/data`, but product code uses
the logical paths described below rather than depending on arbitrary session
root contents.

## Session Host Layout

The host session directory created by `Store.CreateSession` includes:

```text
<session>/
  context/
  home/
  runtime/
  workspace/
  state/
  logs/
  vm/
  proxy/
  metadata.json
  vm/runtime.json
  proxy/jupyter.json
  state/cells.json
  state/events.json
```

Guest/runtime actually uses:

| Host path | Guest path | Purpose |
| --- | --- | --- |
| `<session>/workspace` | `/workspace` | Jupyter root, cell cwd, loader command cwd, agent working directory |
| `<session>/state` | `/data/state` | Cell artifacts, loader request/result, agent prompt/schema/provider state |
| `<session>/runtime` | `/data/runtime` | Runtime JS MPI/resource/cache |
| `<session>/logs` | `/data/logs` | Jupyter log |
| `<session>/home` child paths | `/root/...` | Session-local tool config/state for declared home entries |

Not exposed by Docker fine-grained mounts:

- `<session>/context`
- `<session>/vm`
- `<session>/proxy`
- `<session>/metadata.json`

## Guest Path Defaults

Default guest paths:

| Config field | Default |
| --- | --- |
| `GuestWorkspacePath` | `/workspace` |
| `GuestHomePath` | `/root` |
| `GuestStateRoot` | `/data/state` |
| `GuestRuntimeRoot` | `/data/runtime` |
| `GuestLogRoot` | `/data/logs` |

`GuestHomePath` is a manifest target path and does not mean agent-compose
overrides `HOME`. Runtime does not explicitly inject `HOME`; tools inside the
guest inherit the image default home.

## Manifest File

Before starting or resuming a session, agent-compose writes:

```text
<session>/vm/mount-manifest.json
```

Manifest structure:

```json
{
  "version": 1,
  "driver": "docker",
  "mounts": [
    {
      "hostPath": "/abs/path/to/session/workspace",
      "guestPath": "/workspace",
      "type": "bind",
      "readOnly": false
    }
  ]
}
```

Constraints:

- `version` is currently `1`.
- `driver` is the resolved runtime driver: `docker`, `boxlite`, or
  `microsandbox`.
- `type` currently supports only `bind`.
- `hostPath` and `guestPath` must both be absolute paths.
- All required host sources are created before the manifest is generated.
- Runtime consumers validate the manifest against the expected driver to avoid
  accidentally reusing an old manifest.

## Home Initialization

Before generating the manifest, agent-compose initializes default config under
`<session>/home` and does not overwrite existing targets:

| Asset | Host target |
| --- | --- |
| `assets/.codex` | `<session>/home/.codex` |
| `assets/.claude` | `<session>/home/.claude` |
| `assets/.claude.json` | `<session>/home/.claude.json` |
| `assets/.gitconfig` | `<session>/home/.gitconfig` |

The guest side no longer runs `.codex` copy synchronization logic. Tools still
see `$HOME` as `/root`, but related config and state are persisted by host
session home.

The logical mount list also creates declared home directories used by current
providers, including `.opencode`, `.gemini`, `.config/{claude,Claude,gemini,opencode}`,
and `.local/share/gemini`.

## Driver Differences

Docker supports file bind mounts, so the Docker manifest keeps fine-grained home
subpath mounts, including `.claude.json` and `.gitconfig` file sources.

BoxLite and Microsandbox do not rely on file source mounts. They mount one
directory source only: `<session> -> /data`. With default configuration,
`/data/state`, `/data/runtime`, and `/data/logs` come directly from that mount.
Guest bootstrap creates `/workspace -> /data/workspace`, keeps `/root` as the
image's real directory, and creates symlinks only for declared home entries such
as `/root/.codex -> /data/home/.codex`, `/root/.claude.json -> /data/home/.claude.json`,
and `/root/.gitconfig -> /data/home/.gitconfig`. Other `/root` subpaths are not
guaranteed to persist on directory-only runtimes.

For the detailed driver-specific layout, see
`runtime_mount_manifest_driver_specific_design.md`.

## Runtime Consumers

Each runtime driver reads `<session>/vm/mount-manifest.json`:

- Docker uses `loadRuntimeMountManifest(session, RuntimeDriverDocker)` and
  applies `DOCKER_HOST_SESSION_ROOT` rebase to each source.
- BoxLite uses `loadDirectoryRuntimeMountManifest(session, RuntimeDriverBoxlite)`
  and validates that all sources are directories before calling
  `boxlite_options_add_volume`.
- Microsandbox uses
  `loadDirectoryRuntimeMountManifest(session, RuntimeDriverMicrosandbox)` and
  validates that all sources are directories before constructing
  `microsandbox.Mount.Bind`.

BoxLite and Microsandbox execute `directoryOnlyGuestSessionBootstrapCommand`
after a session starts or is reconnected, before Jupyter readiness checks, and
again before `Exec` / `ExecStream` user commands. Bootstrap failures prevent the
session or command from being treated as ready and return diagnostics that
include driver/session/runtime context plus stdout/stderr summaries.

## Runtime Paths

Guest paths after startup:

- Jupyter root: `/workspace`
- Jupyter log: `/data/logs/jupyter.log`
- Cell/loader command artifacts: `/data/state/cells/...`
- Agent prompt/schema/provider state: `/data/state/agents/...`
- Runtime JS resources/cache/MPI: `/data/runtime/...`

Runtime command and agent env injection:

- `WORKSPACE=/workspace`
- `STATE_ROOT=/data/state`
- `RUNTIME_ROOT=/data/runtime`

No longer injected:

- `HOME`
- `SESSION_WORKSPACE`
- guest-side `SESSION_ROOT`
