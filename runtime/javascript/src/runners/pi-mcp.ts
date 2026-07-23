import fs from "node:fs/promises";
import path from "node:path";
import { flattenEnvMap } from "../mcp-config.js";

export const piMCPAdapterExtension = "/usr/local/share/agent-compose/pi-mcp-adapter/index.ts";

export async function writePiMCPConfig(
  invocationDir: string,
  configured?: Record<string, unknown>,
): Promise<string | undefined> {
  if (!configured || Object.keys(configured).length === 0) return undefined;

  const mcpServers: Record<string, unknown> = {};
  for (const [name, value] of Object.entries(configured)) {
    if (!value || typeof value !== "object" || Array.isArray(value)) continue;
    const server = value as Record<string, unknown>;
    if (server.type === "local") {
      mcpServers[name] = {
        command: server.command,
        args: Array.isArray(server.args) ? server.args : [],
        env: flattenEnvMap(server.env as Record<string, { value: string }> | undefined),
        cwd: server.cwd,
        lifecycle: "lazy",
      };
    } else if (server.type === "remote") {
      mcpServers[name] = {
        url: server.url,
        headers: flattenEnvMap(server.headers as Record<string, { value: string }> | undefined),
        lifecycle: "lazy",
      };
    }
  }
  if (Object.keys(mcpServers).length === 0) return undefined;

  const configPath = path.join(invocationDir, "mcp.json");
  await fs.writeFile(configPath, JSON.stringify({
    settings: { sampling: false, elicitation: false, outputGuard: true },
    mcpServers,
  }, null, 2) + "\n", { encoding: "utf8", mode: 0o600 });
  return configPath;
}
