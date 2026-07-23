import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import { writePiMCPConfig } from "../src/runners/pi-mcp.js";

describe("Pi MCP configuration", () => {
  const roots: string[] = [];
  afterEach(async () => Promise.all(roots.splice(0).map((root) => fs.rm(root, { recursive: true, force: true }))));

  it("maps local and remote servers into a private adapter config", async () => {
    const root = await fs.mkdtemp(path.join(os.tmpdir(), "pi-mcp-test-"));
    roots.push(root);
    const configPath = await writePiMCPConfig(root, {
      local: { type: "local", command: "mcp-server", args: ["--stdio"], cwd: "/workspace", env: { TOKEN: { value: "secret", secret: true } } },
      remote: { type: "remote", transport: "sse", url: "https://mcp.example/sse", headers: { Authorization: { value: "Bearer token", secret: true } } },
    });

    expect(configPath).toBe(path.join(root, "mcp.json"));
    expect((await fs.stat(configPath!)).mode & 0o777).toBe(0o600);
    expect(JSON.parse(await fs.readFile(configPath!, "utf8"))).toEqual({
      settings: { sampling: false, elicitation: false, outputGuard: true },
      mcpServers: {
        local: { command: "mcp-server", args: ["--stdio"], env: { TOKEN: "secret" }, cwd: "/workspace", lifecycle: "lazy" },
        remote: { url: "https://mcp.example/sse", headers: { Authorization: "Bearer token" }, lifecycle: "lazy" },
      },
    });
  });

  it("does not write a config when no supported server is configured", async () => {
    const root = await fs.mkdtemp(path.join(os.tmpdir(), "pi-mcp-test-"));
    roots.push(root);
    await expect(writePiMCPConfig(root, { invalid: { type: "unknown" } })).resolves.toBeUndefined();
    await expect(fs.readdir(root)).resolves.toEqual([]);
  });
});
