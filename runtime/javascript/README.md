# @chaitin-ai/agent-compose-runtime

`@chaitin-ai/agent-compose-runtime` is the guest-side runtime package used by agent-compose agent sandboxes. It exposes the compatible CLI entrypoint:

```sh
agent-compose-runtime prompt \
  --provider <codex|claude|gemini|opencode|pi> \
  --message-file <path> \
  --output-schema-file <path> \
  --state-root <path> \
  --workspace <path> \
  --home <path>
```

Successful runs write a single structured result line to stdout with the `__AGENT_RESULT__` prefix. Human-readable agent transcript output is written to stderr.

`--output-schema-file` is optional. When set, the file must contain a JSON Schema object. The runtime passes it to the provider's native structured-output mechanism where supported. Codex and Claude support schema-based output; Gemini, OpenCode, and Pi currently reject schema requests until a native provider mechanism is wired.

## Agent system prompt (convention path)

When the host binds a run to an agent definition with non-empty `system_prompt`, it writes:

```text
<state-root>/agents/system-prompts/system-prompt.txt
```

The `prompt` command reads that convention path, combines it with the MPI catalog via `buildSystemContext` in `src/system-context.ts`, and passes the result to provider runners as `systemContext`. Per-turn user text stays in `--message-file` only.

See `docs/design/agent_system_prompt_design.md` for the full host/guest contract.

## Development

```sh
npm install
npm run build
npm test
```

The TypeScript source lives in `src/`:

- `cli.ts`: commander-based CLI.
- `prompt.ts`: command orchestration and default path resolution.
- `system-context.ts`: agent identity + MPI composition.
- `runners/`: provider adapters for Codex, Claude, Gemini, OpenCode, and Pi.
- `mpi.ts`: MPI catalog discovery and context formatting.
- `session-state.ts`: provider thread resume state persistence.
