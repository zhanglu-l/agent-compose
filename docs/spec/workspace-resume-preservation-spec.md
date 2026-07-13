# Workspace Resume 状态保持技术规格

## 背景与目标

agent-compose 的 sandbox workspace 是宿主机 `SANDBOX_ROOT` 下的可写目录，并通过 runtime driver 映射到 guest `/workspace`。它不是只读目录，也不是 runtime 停止时自动丢弃的临时层。

当前问题出在 workspace provisioning 与 sandbox resume 共用了同一条准备路径：

- sandbox 创建时调用 `workspaces.PrepareSessionWorkspace` 初始化 workspace。
- v1 Session resume、Jupyter 自动恢复、project run 复用和 loader sticky sandbox resume 也会再次调用同一函数。
- file workspace 的 `Prepare` 会把模板根目录逐项复制到 sandbox workspace；复制目录或文件前会先删除目标同名路径。
- project `local` provider 会先生成 file workspace snapshot，因此具有相同的重复复制行为。
- Git workspace 通过“workspace 是否非空”跳过 clone，但该判断不是持久化生命周期状态；用户把 workspace 清空后，resume 仍可能重新 clone。

这使 resume 的可观察语义取决于 provider：Git workspace 通常保留状态，而 file/local workspace 中与模板重名的修改、删除或重命名可能在 resume 时被覆盖或复活。该行为与 sandbox resume 应恢复先前状态的含义冲突。

本规格的目标状态：

- workspace provisioning 是 sandbox 首次启动前的一次性初始化，而不是每次 runtime 启动前的同步操作。
- 初始化成功的 workspace 在 stop/resume、daemon restart、Jupyter 自动恢复、sticky loader 和显式 sandbox 复用过程中保持原样。
- resume 不读取当前 workspace config、file/local 模板或 Git 远端，也不向 sandbox workspace 写入任何控制面内容。
- 初始化失败可以重试；只有从未成功初始化的 workspace 才允许重新 materialize。
- 新 sandbox 继续从创建时解析到的最新 file/local 模板或 Git 源初始化。
- 旧 sandbox 升级后原样保留，不发生一次性的破坏性重新初始化。
- sandbox workspace 的修改不回写 file/local 模板，也不自动 commit 或 push Git 源。

## 现状和 harness 约束

### 项目边界

`AGENTS.md` 定义 agent-compose 是 sandbox 控制面，workspace 相关边界包括：

- `pkg/storage/sessionstore`：创建和持久化 sandbox metadata，以及 `<sandbox>/workspace`、`state`、`runtime`、`home` 等目录。
- `pkg/workspaces`：file/Git workspace config 解析和 materialization。
- `pkg/sessions`：通用 stop、resume 和 Jupyter proxy readiness 生命周期。
- `pkg/agentcompose/adapters`：v1 Session、loader sandbox 和 runtime driver 适配。
- `pkg/runs`：project run 的 local/Git workspace snapshot、sandbox 创建和复用。
- `pkg/driver`：Docker、BoxLite、Microsandbox 的 runtime mount 和启动停止行为。

workspace 的宿主机路径为：

```text
<SANDBOX_ROOT>/<sandbox-id>/workspace
```

Docker 将该目录直接 bind mount 到 guest workspace；BoxLite 和 Microsandbox 将 sandbox 根目录映射到 guest `/data`，再暴露 `/workspace`。逻辑 workspace mount 默认可写。本规格不改变三种 driver 的挂载协议。

### 当前生命周期事实

`pkg/storage/sessionstore/store.go` 在创建 sandbox 时预创建 workspace 目录，并将路径保存到 `SandboxSummary.WorkspacePath`。stop 只更新 runtime 和 sandbox 状态；`RemoveSandbox` 才删除整个 sandbox 目录。

`pkg/workspaces/workspace.go` 当前根据 sandbox 保存的 workspace snapshot 或 workspace ID 解析 config，然后调用 file/Git provider 的 `Prepare`。以下入口都会触发该逻辑：

- v1 `CreateSession` 和 `ResumeSession`。
- `sessions.Lifecycle.EnsureProxyReady`。
- `LoaderSandboxRunner` 的 create 和 `LoadOrResume`。
- project run 创建或复用 sandbox 时的 `startProjectRunSandbox`。

`pkg/workspaces/file_workspace.go` 的复制不是 merge：模板中的每个目标文件或目录会被删除后重新创建；模板中不存在的其他顶层路径可能被偶然保留。这既不是完整 reset，也不是状态保持。

`pkg/workspaces/git_workspace.go` 当前把“除内部目录外存在任意 entry”视作已经初始化。目录内容不是可靠的生命周期标记：空模板、用户主动清空、失败的半成品 clone 和完整 workspace 无法由该启发式稳定区分。

### 测试和质量门禁

`TESTING.md` 要求：

- unit tests 验证状态机、序列化、错误路径和并发等局部行为。
- integration tests 验证文件持久化、生命周期组件和受控本地依赖协作。
- E2E tests 验证通过正式 API 和真实 runtime 完成的用户工作流。

该变更跨越 sandbox metadata、文件持久化、session/run/loader 生命周期和 runtime resume，必须同时具有 unit、integration 和真实 Docker E2E 证明。

`Taskfile.yml` 中的强制本地门禁为：

```bash
task lint
task test
task build
```

`task test` 通过 `scripts/test-coverage.sh` 汇总 unit、integration、E2E 和 combined coverage，并执行 `TESTING.md` 的最低覆盖率要求。真实 Docker runtime E2E 依照仓库现有约定使用显式 opt-in task，不加入普通 `task test` 或当前 GitHub-hosted CI。

## 核心概念和领域模型

### Workspace Source

Workspace Source 是 sandbox workspace 的初始化来源：

- file workspace：`DATA_ROOT/workspaces/<workspace-id>/content` 下的持久化模板。
- project local workspace：在 project run 准备阶段从 project source path 生成的 run 专属 file workspace snapshot。
- Git workspace：保存于 sandbox workspace snapshot 中的 URL、branch、commit 和 clone target。

Workspace Source 只参与首次 provisioning。它不是已创建 sandbox 的持续同步源。

### Sandbox Workspace

Sandbox Workspace 是单个 sandbox 独占的可写工作副本，路径为 `SandboxSummary.WorkspacePath`。其生命周期与 sandbox 一致：

- create：创建空目录并在需要时执行 provisioning。
- stop：保留目录及内容。
- resume：复用原目录及内容。
- remove/prune：随整个 sandbox 目录删除。

同一 workspace config 可以初始化多个互相隔离的 sandbox workspace。一个 sandbox 内的修改不传播到其他 sandbox 或 Workspace Source。

### Workspace Provisioning

Workspace Provisioning 是把 Workspace Source materialize 为 Sandbox Workspace 的一次性过程。它必须在首次 runtime 启动前成功并持久化完成状态。

Provisioning 不属于 resume。进入完成态后没有隐式反向转换；只有删除 sandbox 并新建 sandbox 才会再次应用 Workspace Source。

### Resume

Resume 是恢复同一个 sandbox 的 runtime 执行能力。对 workspace 的合同是：

```text
workspace_after_resume == workspace_before_stop
```

等价性至少包括：

- 路径集合。
- 文件内容。
- 文件类型和权限。
- 目录结构。
- symlink 本身及其 target。

测试不要求保持访问时间等不稳定文件系统元数据，但实现不得主动触碰或重写 ready workspace 中的任何路径。

### Provisioning 状态机

带 workspace 的 sandbox 持久化以下状态：

| 状态 | 含义 | 允许的后续转换 |
| --- | --- | --- |
| `pending` | sandbox 已创建，但 workspace 尚未完成初始化 | `ready`、`failed` |
| `failed` | 最近一次首次初始化失败，runtime 从未基于该 workspace 启动 | `pending` |
| `ready` | workspace 已完成初始化，或旧 sandbox 已按兼容策略认定完成 | 无 |

合法转换为：

```text
new sandbox: pending -> ready
                    \-> failed -> pending -> ready|failed

legacy sandbox: <missing> -> ready
```

`ready` 是 sandbox 生命周期内的终态。本规格不定义 `ready -> pending`、refresh 或 reset。

## 架构和组件边界

### Workspace Provisioner

`pkg/workspaces` 新增进程级单例 Provisioner，作为所有 workspace 生命周期入口的唯一编排者。Provisioner 依赖：

- application config。
- workspace config store。
- sandbox store 的 `GetSandbox` 和 `UpdateSandbox` 能力。
- provider materializer。

Provisioner 对外提供等价于以下语义的接口：

```go
Ensure(ctx context.Context, sandbox *model.Sandbox) error
```

`Ensure` 必须：

1. 按 sandbox ID 串行化同一 sandbox 的 provisioning。
2. 在进入临界区后重新加载持久化 sandbox metadata，避免依据调用方的陈旧状态执行初始化。
3. 对无 workspace 的 sandbox 直接返回。
4. 对 legacy 缺省状态执行兼容迁移，不读取 Workspace Source。
5. 对 `ready` 直接返回，不解析 workspace config、不检查目录内容。
6. 只对 `pending` 或 `failed` 执行 materialization。
7. 在返回成功前持久化 `ready`，保证 runtime 不会先于完成状态启动。
8. 把重新加载和更新后的持久化字段同步回调用方 sandbox，同时保留 `RuntimeEnvItems`、`ProviderEnvItems` 等 transient 字段。

并发控制使用 sandbox ID 作为 key。实现可以使用 `golang.org/x/sync/singleflight`；若采用该包，应将现有 indirect dependency 提升为 direct dependency。并发调用者在共享执行结果后必须重新读取最终 metadata，不能只共享第一个调用者的内存对象。

### Provider Materializer

file 和 Git provider 继续负责解析各自 config 并把内容写入指定目标目录，但不再负责判断 sandbox 是否处于 create 或 resume。

provider materializer 只由 Provisioner 在 `pending/failed` 状态下调用。生命周期代码不得直接调用 materializer。

现有 `HostWorkspaceInitialized` 之类的目录内容启发式不得再作为跳过 resume 或决定是否 clone 的依据。用户把 ready Git workspace 清空后，resume 必须保持空目录，不得重新 clone。

### Staging 和提升

首次 provisioning 使用 sandbox 内同一文件系统上的 staging 目录，例如：

```text
<sandbox>/state/workspace-provisioning/attempt-<id>/
```

流程为：

1. 清理同一 sandbox 遗留的未完成 staging attempt。
2. 在新的 staging 目录中完成 file copy 或 Git clone/checkout。
3. materialization 全部成功后，把 staging 提升为正式 `WorkspacePath`。
4. 提升成功后持久化 `ready`。

正式 workspace 在 `ready` 后不参与任何 staging 清理。`pending/failed` sandbox 尚未成功启动，重试时可以丢弃此前的半成品 workspace 或 staging。

staging 和正式 workspace 位于同一 sandbox 根目录，避免跨文件系统 rename。若提升或状态持久化失败，runtime 不得启动；下次重试仍按未完成初始化处理。

### 生命周期入口

以下入口必须注入并调用同一个 Provisioner 实例：

- v1 Session create 和 resume。
- `sessions.Lifecycle.ResumeLoaded`。
- `sessions.Lifecycle.EnsureProxyReady`。
- loader sandbox create 和 `LoadOrResume`。
- project run 创建新 sandbox 和复用已有 sandbox 的启动路径。

create 和 resume 可以统一调用 `Ensure`，但实际行为由持久化状态决定。不得依赖调用方传入 `isResume` 布尔值决定是否复制，因为 daemon restart、失败重试和多个入口会使调用上下文不可靠。

capability catalog、agent provider state、runtime facade 配置等受管状态继续写入 sandbox 的 `runtime/`、`state/` 或 `home/`。这些目录不属于 Sandbox Workspace，允许按现有生命周期规则刷新。

### Session Store

`pkg/storage/sessionstore` 仍是 sandbox metadata owner。创建带 workspace snapshot 或非空 workspace ID 的 sandbox 时，必须在首次保存 metadata 时写入 `pending`。

Store 不执行 file copy 或 Git clone，也不包含 provider 逻辑。它只负责：

- 初始化状态字段。
- 向后兼容地读写缺省字段。
- 继续在 sandbox remove 时删除 metadata、workspace 和 provisioning staging。

## API、CLI、配置和数据模型

### Sandbox metadata

`model.Sandbox` 新增可选字段，持久化 JSON 形态固定为：

```json
{
  "workspace_provisioning": {
    "version": 1,
    "status": "pending",
    "updated_at": "2026-07-13T12:00:00Z"
  }
}
```

对应领域类型应包含：

```go
type SandboxWorkspaceProvisioning struct {
    Version   int       `json:"version"`
    Status    string    `json:"status"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

约束：

- 当前只接受 `version == 1`。
- status 只接受 `pending`、`ready`、`failed`。
- `updated_at` 记录最近一次状态转换时间，不宣称是旧 sandbox 的真实初始 provisioning 时间。
- 无 workspace 的 sandbox 不写该字段。
- 该字段不加入 `SandboxSummary`，不参与列表过滤。

### 公开接口

本规格不修改：

- `proto/agentcompose/v1` 和 `proto/agentcompose/v2`。
- Connect/HTTP wire shape。
- `@chaitin-ai/agent-compose-client`。
- CLI 命令、flag 或输出。
- daemon 应用环境变量。
- Docker Compose 或镜像默认值。
- SQLite schema。

Provisioning 状态是内部恢复合同，不要求通过公开 API 暴露。错误继续通过现有 Create/Resume/Run 接口返回，并按现有逻辑把 sandbox 标记为 failed。

### Workspace snapshot

创建 sandbox 时保存的 `SandboxWorkspace` snapshot 继续是该 sandbox 的初始化事实源。workspace config 后续被修改或删除，不改变已经 `ready` 的 sandbox。

新 sandbox 使用创建时解析到的最新 config：

- file workspace 读取最新模板内容。
- project local workspace 使用新 run 的 snapshot。
- Git workspace 使用新 sandbox snapshot 中的 URL、branch 和 commit。

显式复用已有 sandbox 时，以已有 sandbox 保存的 workspace 和 provisioning 状态为准。本次 run spec 不能替换或刷新它。

### 旧数据迁移

旧 metadata 中带 workspace、但没有 `workspace_provisioning` 字段的 sandbox，一律视为已经初始化：

1. 加载现有 metadata 和 workspace。
2. 写入 `version: 1`、`status: ready` 和当前 `updated_at`。
3. 状态持久化成功后继续 resume。
4. 不解析 workspace config，不读取 file/local 模板或 Git 源，不检查 workspace 是否为空。

该策略适用于旧 sandbox 当前为 pending、running、stopped 或 failed 的情况。它优先保证已有文件不被破坏；极少数历史上从未成功完成初始化的 sandbox 可能保持空或半成品状态，用户应删除后新建 sandbox。本规格不通过自动重建牺牲已有数据安全。

未知 version、未知 status 或非法状态组合不进行兼容猜测。Provisioner 返回错误，workspace 保持原样。

## 工作流和失败语义

### 新 sandbox 首次启动

1. sessionstore 创建 sandbox 目录和 metadata。
2. 有 workspace 时保存 `pending`；无 workspace 时不创建 provisioning 状态。
3. Provisioner 在 staging 目录执行 materialization。
4. 成功后提升 staging，并保存 `ready`。
5. capability/runtime/home 等受管准备继续执行。
6. runtime driver 启动 sandbox。
7. sandbox 状态更新为 running。

在第 4 步完成前不得进入第 6 步。

### Ready sandbox resume

1. resume 入口加载 sandbox。
2. Provisioner 重新加载 metadata 并观察到 `ready`。
3. Provisioner 不调用 config store 和 provider materializer，不访问 `WorkspacePath`。
4. runtime driver 恢复同一 sandbox runtime。
5. sandbox 状态更新为 running。

source 模板变更、workspace config 删除、Git 远端不可达都不影响该流程。

### 首次 provisioning 失败

- file copy、Git clone/checkout、staging 提升失败时，Provisioner 将状态保存为 `failed` 并返回原始错误。
- 若保存 `failed` 也失败，返回值必须同时保留原始 provisioning 错误和持久化错误。
- runtime 不启动；sandbox 按现有调用路径标记为 failed。
- 后续 resume/重试将 `failed` 转为 `pending`，清理半成品 staging 后重新 materialize。
- 因为该 workspace 从未进入 `ready` 且 runtime 未成功启动，重试不属于对有效 resume 状态的破坏。

### Runtime 启动失败

workspace 已进入 `ready` 后，runtime 启动仍可能失败。该情况下：

- workspace 保持 `ready`。
- sandbox VM status 可以按现有逻辑标记为 failed。
- 后续 resume 只重试 runtime 启动，不重新 materialize workspace。

### 状态持久化失败

- materialization 成功但 `ready` 保存失败时，Provisioner 返回错误，runtime 不启动。
- metadata 仍为 `pending/failed` 时，下次尝试可以重新 materialize，因为没有 runtime 基于该 workspace 成功运行。
- legacy migration 写入 `ready` 失败时，同样终止 resume，但不得修改 workspace。

### 并发调用

同一 sandbox 的多个 create/resume/EnsureProxyReady 请求只能有一个 provisioning 执行者。等待者共享结果后重新读取 metadata：

- 首个调用成功时，其他调用观察到 `ready`。
- 首个调用失败时，其他调用收到失败或基于持久化 `failed` 重新进入显式重试，不得并行复制。
- 不同 sandbox 可以并行 provisioning。

### Stop、remove 和 daemon restart

- stop 不修改 provisioning 状态和 workspace。
- daemon graceful shutdown 或进程退出不修改 provisioning 状态。
- daemon 使用相同 `DATA_ROOT`/`SANDBOX_ROOT` 重启后，从 metadata 恢复状态。
- remove/prune 继续删除整个 sandbox 目录，包括 workspace、metadata 和 staging。
- `REMOVE_ON_COMPLETION` 仅按当前规则删除本次创建的 sandbox；对复用 sandbox 的保护行为不变。

## 测试、质量门禁和验收标准

### Unit tests

#### Metadata 和状态机

- 创建带 workspace snapshot 的 sandbox，首次持久化状态为 `pending`。
- 仅提供 workspace ID 的 sandbox 同样为 `pending`。
- 无 workspace 的 sandbox 不产生 provisioning 状态。
- save/load 和 Store 重建后状态、version、timestamp 保持。
- 旧 metadata 没有新字段时可以正常反序列化。
- 合法状态转换通过；`ready -> pending`、未知 status/version 被拒绝。

#### Provisioner

- file workspace 首次 `pending -> ready`，模板只读取和复制一次。
- `ready` 后再次 Ensure：即使模板改变、config store 返回 not found 或 materializer 被设置为必然失败，也必须成功并保持 workspace。
- 用户修改模板同名文件、删除模板文件、重命名目录、新增文件或 symlink 后，Ensure 前后的 workspace manifest 相同。
- Git workspace 首次 clone 一次；本地修改、删除全部 tracked files 或远端新增 commit 后，ready Ensure 不再 clone。
- initial materialization 失败保存 `failed`；修复 source 后重试可以进入 `ready`。
- runtime 启动失败后的 ready workspace 不重试 materialization。
- legacy 缺省状态直接写 ready，materializer 调用次数为零，空目录也不例外。
- 未知状态 fail closed，workspace 内容不变。
- 同一 sandbox 并发 Ensure 只调用一次 materializer；不同 sandbox 可并行。
- staging 失败、遗留 attempt、提升失败和 metadata 保存失败具有确定性清理与错误结果。

### Integration tests

使用临时 `DATA_ROOT`、真实 sessionstore/configstore 和 fake runtime driver，至少覆盖：

1. v1 Session file workspace：create 后修改已有文件、删除模板文件、新增文件，stop 后改变模板，再 resume；比较 resume 前后的 path/type/mode/content/symlink manifest。
2. Jupyter `EnsureProxyReady`：stopped ready sandbox 自动恢复时不调用 materializer，driver 启动前 workspace 已保持。
3. loader sticky sandbox：第二次 `LoadOrResume` 保留第一次运行产生的 workspace 状态。
4. project local workspace：
   - run A 创建 sandbox 并修改 workspace。
   - project source 改变后显式复用 run A sandbox，原状态保持。
   - 新建 run B sandbox，得到更新后的 project source snapshot。
5. daemon persistence：释放并重建 Store/Provisioner 实例后 resume，证明状态和 workspace 均来自持久化数据。
6. legacy migration：手工写入缺少新字段的旧 metadata，首次 resume 只补写 ready，不解析已删除的 workspace config。
7. 首次失败重试：materialization 失败时 driver start 调用次数为零；修复 source 后重试只启动一次。

每个生命周期入口应有测试证明它调用统一 Provisioner，而不是直接 materialize。可通过 fake Provisioner 的调用计数和“ready 时 materializer 必须 panic”防止未来绕过。

### 真实 Docker E2E

在 `test/e2e` 新增：

```text
TestE2EDockerFileWorkspaceResumePreservesState
```

该测试必须使用编译后的真实 daemon、正式 Connect/HTTP API 和真实 Docker guest：

1. 启动独立 daemon，使用临时 `DATA_ROOT` 和 `SANDBOX_ROOT`。
2. 通过 v1 `ConfigService.CreateWorkspaceConfig` 创建 file workspace。
3. 通过正式 workspace upload HTTP API 上传 `modified.txt` 和 `deleted.txt` 模板。
4. 通过 v1 `SessionService.CreateSession` 创建 Docker sandbox。
5. 通过 v2 `ExecService.Exec`：
   - 修改 `modified.txt` 为 agent 版本。
   - 删除 `deleted.txt`。
   - 新增 `generated.txt`。
6. 通过 v1 `StopSession` 停止 sandbox。
7. 通过 upload API 把模板 `modified.txt` 改为新的 source 版本，并确认模板没有 `generated.txt`。
8. 停止 daemon，使用相同 data root 重启 daemon。
9. 通过 v1 `ResumeSession` 恢复原 sandbox。
10. 通过 Exec 断言：
    - `modified.txt` 仍是 agent 版本。
    - `deleted.txt` 没有复活。
    - `generated.txt` 仍存在。
    - runtime 和 sandbox ID 没有改变。
11. 新建第二个 sandbox，断言它获得新的 source 版本和 `deleted.txt` 模板，但没有第一个 sandbox 的 `generated.txt`。
12. 通过 workspace download/list API 断言第一个 sandbox 的修改没有回写模板。
13. 通过公开 remove API 清理 sandbox，并执行 Docker fallback leak cleanup。

新增聚焦任务：

```bash
task test:e2e:docker-workspace-resume
```

该任务遵循现有 `test:e2e:docker-jupyter` 模式：依赖 daemon build，检查显式配置或默认的本地 guest image，不在测试过程中隐式拉取镜像。E2E helper 应复用 daemon process、端口、client、日志和 cleanup 逻辑，避免复制新的生命周期 harness。

按已确认范围，本次不新增 BoxLite/Microsandbox 真实 E2E；driver 无关的 provisioning 语义由 unit/integration 覆盖，Docker E2E 证明公开 API、持久化和真实 runtime 的完整链路。

### 文档和门禁

实现完成时同步更新：

- `docs/design/agent-compose_design.md` 及中文版本：workspace seed、ready 和 resume 合同。
- `docs/spec/core-e2e-test-strategy-spec.md`：将 workspace resume 保持纳入对应场景验收。
- `TESTING.md`：新增 Docker workspace resume 聚焦任务说明。
- `Taskfile.yml`：注册 opt-in E2E task。

focused tests 通过后，最终执行：

```bash
task test:e2e:docker-workspace-resume
task lint
task test
task build
```

### 验收标准

- 对任意 `ready` file、local 或 Git workspace，所有 resume 入口均不会调用 provider materializer。
- stop/resume 和 daemon restart 前后的 sandbox workspace manifest 一致。
- 修改 Workspace Source 不影响已有 sandbox；新 sandbox 使用最新 source。
- 删除 workspace config 不阻止 ready sandbox resume。
- 旧 sandbox 第一次升级 resume 不修改 workspace，即使 workspace 为空或 sandbox 当前为 failed。
- 首次 provisioning 未成功时 runtime 不启动；修复 source 后可以重试。
- remove/prune 仍完整删除 sandbox workspace 和 provisioning 状态。
- 无公开 API、CLI、proto、配置或部署兼容性变化。
- `task lint`、`task test`、`task build` 和 opt-in Docker workspace E2E 全部通过。

## 首版不做事项

- 不提供 workspace reset、refresh、reseed 或 force-reinitialize API/CLI。
- 不把 sandbox 修改同步回 file/local Workspace Source。
- 不自动 commit、push、pull、fetch 或 merge Git workspace。
- 不允许 project spec 或 workspace config 更新隐式改变已有 sandbox。
- 不在公开 Session/Sandbox API 暴露 provisioning 状态。
- 不引入 SQLite migration。
- 不改变 runtime driver mount、rootfs writable layer 或 volume mount 语义。
- 不为 BoxLite/Microsandbox 新增真实 provisioning E2E；后续可在完整核心 E2E 体系中扩展 driver 矩阵。
- 不解决多个 daemon 进程同时共享并写入同一 `SANDBOX_ROOT` 的分布式锁问题。

## 关键假设和已确认决策

- resume 的首要合同是保持同一个 sandbox 的先前状态，而不是同步最新 Workspace Source。
- 已有 sandbox 没有 provisioning 字段时，一律原样迁移为 ready，不按 VM status 或目录内容重新初始化。
- 需要最新模板的用户创建新 sandbox；首版不提供显式 refresh/reset。
- 真实 runtime 回归采用 Docker E2E，加 driver 无关的 unit/integration 覆盖；不要求本次运行三 driver smoke。
- 每个 sandbox 由单个 daemon 实例拥有；同一进程内并发由 Provisioner 串行化。
- sandbox 保存的 workspace snapshot 在创建后不可变，是首次 provisioning 的事实源。
- `runtime/`、`state/`、`home/` 是控制面受管持久化目录，可以在 resume 时按各自合同更新；本规格的“不写入”约束仅针对 `WorkspacePath`。
