# agent-compose CLI 改进计划

本文面向研发和评审，记录 agent-compose CLI 从当前代码状态迁移到目标命令体系的实施计划。最终用户文档见 [命令行使用手册](../command-line-manual.md)。

## 当前代码依据

当前 CLI 入口集中在 [cmd/agent-compose/main.go](/data/src/github.com/kingfs/agent-compose/cmd/agent-compose/main.go:405) 的 `newRootCommand`。

已实现全局参数：

- `--host`
- `-f, --file`
- `--project-name`
- `--json`

当前 project 解析逻辑：

- `resolveComposePath` 只在未指定 `-f` 时读取当前目录下的 `agent-compose.yml`。
- `loadNormalizedCompose` 使用 `compose.ParseFile` 解析配置，并用 `--project-name` 覆盖配置中的 project name。
- `runComposeUpCommand` apply project 时会把 `ProjectSource.ComposePath` 设置为配置文件路径，把 `ProjectSource.ProjectDir` 设置为配置文件所在目录。

当前已注册命令：

| 命令 | 当前用法 | 当前实现 |
| --- | --- | --- |
| `daemon` | `agent-compose daemon` | 启动 daemon。 |
| `version` | `agent-compose version` | 输出 build version。 |
| `status` | `agent-compose status` | 请求 daemon version/status。 |
| `config` | `agent-compose config [--quiet]` | 解析并输出 normalized config。 |
| `up` | `agent-compose up` | 调用 v2 `ProjectService.ApplyProject`；无 `-d/--detach`，无前台 attach。 |
| `down` | `agent-compose down` | 调用 v2 `ProjectService.RemoveProject`。 |
| `run` | `agent-compose run <agent> [prompt...]` | 调用 v2 `RunService.RunAgentStream`；positional 剩余参数会拼成 prompt；支持 `--prompt`、`--session-id`、`--keep-running`。 |
| `logs` | `agent-compose logs` | 支持 `--agent`、`--run-id`、`--session-id`、`--follow`。 |
| `ps` | `agent-compose ps` | 通过 `GetProject` 汇总 project agents、latest run 和 running session；当前不是 sandbox 列表。 |
| `exec` | `agent-compose exec [flags] <command> [args...]` | 调用 v2 `ExecService.ExecStream`；通过 `--agent`、`--run-id`、`--session-id` 选择目标；支持 `--cwd`。 |
| `inspect` | `agent-compose inspect <project|agent|run|session> [name-or-id]` | 查看 project、agent、run、session。 |
| `images` | `agent-compose images` | 调用 image list；支持 `--query`、`-a/--all`。 |
| `pull` | `agent-compose pull <image>` | 调用 image pull；支持 `--platform`。 |
| `rmi` | `agent-compose rmi <image>` | 调用 image remove；支持 `--force`、`--prune-children`。 |
| `image` | `agent-compose image <subcommand>` | 旧 image 命令树，包含 `ls`、`pull`、`rm`、`inspect`。 |

现有后端/API 能力：

- v2 `ProjectService` 已有 `ListProjects`、`ApplyProject`、`GetProject`、`RemoveProject`、`WatchProject`。
- v2 `ListProjectsResponse` 返回 `ProjectSummary` 列表，summary 包含 `project_id`、`name`、`source_path`、`current_revision`、`spec_hash`、`agent_count`、`scheduler_count`、`running_run_count`、`latest_run_id`、`created_at`、`updated_at`、`removed_at`。
- 当前 `ProjectService.ListProjects` 调用 `ProjectSummaryToProto(project, nil, nil)`，因此 `agent_count` 和 `scheduler_count` 在 list 场景下会是 0；如果 `ls` 要展示真实 agent/scheduler 数量，需要修复该实现或补充查询。
- v1 `SessionService` 已有 `ResumeSession`、`StopSession`、`GetSession`、`ListSessions`、`WatchSession`，没有删除 session/sandbox 的 RPC。
- runtime driver 已有 stop 能力；资源统计命令没有现成 CLI 和统一 API。

## 目标命令体系

目标用户文档以 sandbox 为统一对象。研发实现时可以继续复用已有 session/run 数据结构，但 CLI 参数、输出字段和错误信息应对外统一使用 sandbox。

最终顶层命令：

```text
daemon
version
status
config
ls
up
down
run
ps
stop
resume
rm
exec
logs
inspect
stats
images
pull
push
rmi
```

本轮不实现 `build`。旧 `image` 命令树迁移到顶层 image 命令和 `inspect image`。

## 分阶段实施计划

### 1. 配置文件发现

目标：

- 显式 `-f` 支持任意路径，常规后缀为 `.yml` 和 `.yaml`。
- 未指定 `-f` 时，在当前目录查找 `agent-compose.yml` 和 `agent-compose.yaml`。
- 如果两个文件同时存在，返回 usage error，要求用户显式指定 `-f`。

代码入口：

- `resolveComposePath`
- `loadNormalizedCompose`
- `resolveComposeProject`

测试点：

- 默认发现 `.yml`。
- 默认发现 `.yaml`。
- 两个文件同时存在时报错。
- `-f /path/to/project/agent-compose.yaml` 时 project root 为文件所在目录。
- `--project-name` 仍覆盖 normalized project name。

### 2. 新增 `ls`

目标：

- 新增 `agent-compose ls`，列出 daemon 上所有 project。
- 支持 `--verbose`。
- 支持 `--json`。

代码依据：

- v2 `ProjectService.ListProjects` 已存在。
- `ListProjectsRequest` 支持 `query`、`include_removed`、`offset`、`limit`。
- `ProjectSummary` 已提供大部分需要输出的字段。

实现要点：

- 在 `newRootCommand` 注册 `ls`。
- CLI 端处理分页，至少拉取到 `has_more=false`。
- 默认列建议使用 `PROJECT`、`CONFIG FILE`、`REVISION`、`AGENTS`、`SCHEDULERS`、`SERVICES`。
- `CONFIG FILE` 可先使用 `ProjectSummary.source_path`。如果需要严格区分 compose path 和 project dir，需要检查 `ProjectRecord.Source` 的存储和 `ProjectServiceSourcePath`。
- 修复 `ProjectService.ListProjects` 中 agent/scheduler 数量为 0 的问题，或在 CLI 中避免展示不准确字段。
- 当前 API 没有 `services` 字段；若必须展示，需要先扩展 proto/store 或在本期将 services 显示为 `-` 并在 JSON 中清晰表达不可用。

测试点：

- 空 project 列表。
- 多 project 按更新时间排序。
- `--json` 输出包含分页后的完整列表。
- `--verbose` 包含 project id、source path、spec hash、created/updated/removed。

### 3. `inspect` 迁移

目标：

- 新增 `inspect image <image>`。
- 新增 `inspect sandbox <sandbox>`。
- 保留 `inspect project|agent|run`。
- `inspect session <id>` 暂时作为 alias，输出 deprecation warning。
- `image inspect <image>` 暂时作为 alias，输出 deprecation warning。

代码入口：

- `runComposeInspectCommand`
- `runComposeImageInspectCommand`
- `newRootCommand` 中 `imageCmd` 和 `inspectCmd`

测试点：

- `inspect image` 输出与旧 `image inspect` 一致。
- `inspect sandbox` 输出与旧 `inspect session` 等价，但字段命名对外使用 sandbox。
- 旧命令 warning 写到 stderr，不影响 `--json` stdout。

### 4. `logs` 增强

目标：

- 支持 `logs [agent]`。
- 新增 `-n/--tail`。
- 新增 `-t/--timestamp`。
- 新增 `--sandbox <sandbox>`，旧 `--session-id` 作为 alias。

代码入口：

- `composeLogsOptions`
- `runComposeLogsCommand`
- `followOrPrintProjectLogs`
- `writeLogsForRun`

实现要点：

- positional agent 与 `--agent` 同时出现时报 usage error。
- `--sandbox` 先映射到当前 run/session 查询能力。
- 当前 `--json --follow` 不兼容限制可以保留，直到定义流式 JSON。
- tail 和 timestamp 应在服务端过滤还是 CLI 端过滤需要结合日志来源确定；优先避免读取无限历史后再截断。

测试点：

- `logs reviewer` 等价于 `logs --agent reviewer`。
- `--tail` 对 run detail 和 project logs 都有效。
- `--timestamp` 文本输出包含时间。
- `--sandbox` 与旧 `--session-id` 行为一致。

### 5. `ps` 转为 sandbox 视图

目标：

- `ps` 默认展示 running sandbox。
- `ps -a/--all` 展示全部 sandbox。
- `--status` 过滤 sandbox 状态。
- `--verbose` 展示 driver、image、workspace、Jupyter、错误摘要等。

当前差异：

- 当前 `runComposePSCommand` 基于 project agents 构造 agent 视图。
- 当前输出列是 `AGENT/SCHEDULER/LATEST RUN/RUN STATUS/SESSION/DRIVER/IMAGE`。

实现要点：

- 需要确定 sandbox 数据源：可先由 v1 `ListSessions` + v2 run/project 信息组合得到。
- CLI output adapter 对外字段统一为 sandbox，不暴露 session 作为主列。
- `--all` 需要包含已结束和错误状态；如果现有 session store 已保留历史，可以直接查询，否则需要补 API。

测试点：

- 默认只显示 running。
- `--all` 包含 stopped/exited/error。
- `--status running,error` 正确过滤。
- `--json` 字段稳定。

### 6. `run` 输入模式重构

目标：

- `run <agent> <trigger>` 运行配置中的 trigger。
- `--trigger` 与 positional trigger 等价。
- `--prompt` 是手动 prompt 的唯一入口。
- 新增 `--command`。
- 新增 `-d/--detach`、`-i/--interactive`、`--sandbox`、`--jupyter`、`--jupyter-expose`、`--rm`。
- 旧 `--session-id` 作为 `--sandbox` alias，兼容期后删除。

当前差异：

- 当前 `run <agent> [prompt...]` 会把第二个及后续 positional 参数拼成 prompt。
- 当前 `RunAgentRequest` 有 `Prompt`、`SessionId`、`CleanupPolicy`，没有 trigger/command/detach/jupyter/rm 字段。

实现要点：

- 需要扩展 v2 run API 或定义 trigger/command 到现有 run 模型的映射。
- `--detach` 需要非 streaming 或 stream 早返回语义。
- `--rm` 依赖 sandbox 删除能力。
- `--jupyter`/`--jupyter-expose` 需要 runtime/session 创建参数支持。
- trigger、prompt、command 必须互斥。

兼容策略：

1. 先新增 `--sandbox` alias，保留 `--session-id` warning。
2. 新增 `--trigger`/`--command`，不改变旧 positional prompt。
3. 对旧 positional prompt 输出 warning。
4. 兼容期后将第二 positional 参数解释为 trigger。

测试点：

- `run reviewer --prompt "..."` 不受影响。
- `run reviewer trigger-name` 在迁移期 warning，最终作为 trigger。
- `--trigger`、`--prompt`、`--command` 互斥。
- `--rm --keep-running` 报 usage error。

### 7. `exec` 目标重构

目标：

- `exec <sandbox> [command] [args...]`。
- 新增 `--prompt`、`--command`、`-d/--detach`、`-i/--interactive`。
- 删除 `--agent`、`--run-id`、`--session-id` 目标选择方式。

当前差异：

- 当前第一个 positional 参数是 command。
- 当前通过 `--agent`、`--run-id`、`--session-id` 选择目标。
- 当前支持 `--cwd`。

实现要点：

- 需要调整 Cobra args 解析，避免与旧形式冲突。
- 兼容期内可以通过是否提供旧 target flags 区分旧形式。
- `--cwd` 是否保留为执行上下文参数需要单独决定；如果保留，不应再承担目标选择语义。

测试点：

- `exec sandbox_123 pwd` 目标为 sandbox_123，命令为 pwd。
- 旧 `exec --session-id ... pwd` warning 后仍可用。
- 未传 command 时进入默认交互入口。

### 8. Sandbox 生命周期命令

目标：

- `stop <sandbox...>` 停止 sandbox。
- `resume <sandbox...>` 恢复 sandbox。
- `rm [--force] <sandbox...>` 删除 sandbox。

代码依据：

- v1 `SessionService.StopSession` 和 `ResumeSession` 已存在，可作为 `stop`/`resume` 的第一阶段实现基础。
- 当前没有删除 session/sandbox 的 RPC；`rm` 需要新增后端能力。

`rm` 行为要求：

- 删除非 running sandbox：删除 sandbox 元数据和相关运行态资源。
- 删除 running sandbox 且未传 `--force`：报错，错误信息明确包含 `is running`。
- 删除 running sandbox 且传 `--force`：先停止 sandbox，再删除资源和元数据。
- 多个 sandbox 批量删除时，应逐项返回结果；任一失败时命令返回非零退出码。

实现要点：

- 新增 sandbox 删除 API，或在 v1 SessionService 中新增删除 RPC。
- 明确 running 状态判断来源。
- `--json` 输出应包含每个 sandbox 的 deleted/stopped/error 状态。

测试点：

- running sandbox 不带 `--force` 删除失败。
- running sandbox 带 `--force` 会先 stop 再 delete。
- stopped sandbox 可直接删除。
- 批量删除部分失败时退出码非零，JSON 包含逐项结果。

### 9. `up` 前台/后台语义

目标：

- `up` 默认前台 attach project 输出。
- `up -d/--detach` apply 后返回。
- 前台 `Ctrl+C` 停止整个 project。

当前差异：

- 当前 `up` 只是 `ApplyProject` 后输出 apply 结果，行为更接近目标 `up -d`。

实现要点：

- 第一阶段新增 `--detach`，并让当前行为成为 detach 语义。
- 第二阶段实现 project 级日志 attach。
- 第三阶段处理 signal，调用 project down/stop 逻辑。

测试点：

- `up -d` 返回 project/revision/change summary。
- `up` attach 输出。
- 中断前台 `up` 后 project 被停止。

### 10. `stats`

目标：

- `stats` 默认展示当前 running sandbox 的资源消耗，采集一次后返回。
- `stats -w/--watch` 定期刷新。

当前差异：

- 当前没有 `stats` CLI。
- 当前没有统一 sandbox stats API。

实现要点：

- 先定义统一 stats response。
- Docker runtime 可先接入 Docker stats。
- BoxLite/Microsandbox 指标按可获得性渐进支持，不可用字段显示 `-`。
- watch 模式应支持稳定刷新周期和 Ctrl+C 退出。

测试点：

- 无 running sandbox 时输出空表/空数组。
- running sandbox 有 CPU/memory 字段。
- `--watch` 能定期刷新并响应中断。

### 11. 镜像命令整理

目标：

- 保留顶层 `images`、`pull`、`rmi`。
- 新增 `push`。
- `image inspect` 迁移到 `inspect image`。
- 删除旧 `image` 命令树。

当前依据：

- `images` 已支持 `--query`、`-a/--all`。
- `pull` 已支持 `--platform`。
- `rmi` 已支持 `--force`、`--prune-children`。
- 旧 `image` 命令树与顶层命令重复。

兼容策略：

1. 新增 `inspect image`。
2. 旧 `image ls/pull/rm/inspect` 输出 deprecation warning。
3. 完成兼容期后删除 `image` 命令树。

## 删除项

| 删除项 | 替代方式 | 删除条件 |
| --- | --- | --- |
| `agent-compose image` 命令树 | `images`、`pull`、`rmi`、`inspect image` | `inspect image` 已发布且旧命令 warning 至少一个兼容期。 |
| `agent-compose run <agent> [prompt...]` | `agent-compose run <agent> --prompt "..."` | `--prompt` 文档和 warning 覆盖一个兼容期后。 |
| `agent-compose run --session-id <id>` | `agent-compose run --sandbox <sandbox>` | `--sandbox` 发布后。 |
| `agent-compose exec --agent/--run-id/--session-id ...` | `agent-compose exec <sandbox> ...` | `ps` sandbox 视图稳定后。 |
| `agent-compose logs --session-id <id>` | `agent-compose logs --sandbox <sandbox>` | `--sandbox` 发布后。 |
| `agent-compose inspect session <id>` | `agent-compose inspect sandbox <sandbox>` | `inspect sandbox` 发布后。 |

## 推荐 PR 顺序

1. 文档：命令行使用手册和本改进计划。
2. 配置文件发现：支持 `.yml/.yaml`。
3. `ls`：基于现有 `ListProjects` 增加 CLI，并修复 list project 的 agent/scheduler count。
4. `inspect image` 和 `inspect sandbox`，旧入口 warning。
5. `logs` 增强：positional agent、tail、timestamp、sandbox alias。
6. `ps` sandbox 视图。
7. `stop` 和 `resume`：先基于 v1 session API 实现 sandbox 包装。
8. sandbox 删除 API 与 `rm --force`。
9. `exec <sandbox>` 目标重构。
10. `run` 输入模式重构。
11. `up -d` 与前台 attach/shutdown。
12. `stats` 统一 API 与 CLI。
13. `push` 和旧 `image` 命令树删除。

## 仍需确认

- sandbox id 是否直接等于当前 session id，还是新增独立 alias。无论内部如何实现，CLI 输出和参数都使用 sandbox。
- `run --command` 和 `exec <sandbox> --command` 是否都记录为 run，或 exec 单独记录执行历史。
- `--jupyter` 和 `--jupyter-expose` 需要扩展哪些 session/runtime 创建参数。
- `stats` 的统一采集周期、字段命名和 driver 不支持字段的 JSON 表达方式。
