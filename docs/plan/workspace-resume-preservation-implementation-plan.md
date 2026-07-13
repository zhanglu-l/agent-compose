# Workspace Resume 状态保持实施计划

## 计划目标

本计划落实 `docs/spec/workspace-resume-preservation-spec.md`：把 workspace provisioning 从“每次 runtime 启动前准备”改为“sandbox 首次启动前一次性初始化”，并以持久化状态保证 file、project local 和 Git workspace 在 stop/resume、daemon restart、Jupyter 自动恢复、loader sticky 和显式 sandbox 复用时保持原样。

完成后必须同时满足：

- 新 sandbox 从创建时保存的 workspace snapshot 初始化一次。
- `ready` sandbox 的任何 resume 入口都不读取 Workspace Source、不调用 provider materializer、不写入 `WorkspacePath`。
- 首次初始化失败可以重试；runtime 启动失败不触发 workspace 重建。
- 没有新状态字段的旧 sandbox 原样迁移为 `ready`。
- 新 sandbox 使用最新 source，已有 sandbox 不受 source 更新或删除影响。
- 不新增公开 API、CLI、proto、SQLite schema、环境变量或部署配置。

## 阶段 1：建立 provisioning 持久化状态

目标：sandbox metadata 能准确区分新建未初始化、初始化失败、已初始化和旧格式 sandbox，为后续 Provisioner 提供稳定事实源。

依赖：已确认的 spec；不依赖后续生命周期改造。

实施步骤：

1. 在 `pkg/model` 定义 `SandboxWorkspaceProvisioning`、metadata version `1` 和 `pending`、`ready`、`failed` 常量；在 `Sandbox` 上增加可选 `workspace_provisioning` 字段。
2. 提供集中校验和状态转换 helper：
   - 接受 `pending -> ready|failed`、`failed -> pending`。
   - 把 `ready` 视为终态，拒绝 `ready -> pending|failed`。
   - 拒绝未知 version、未知 status 和 nil sandbox。
   - `updated_at` 在每次合法转换时使用 UTC 当前时间更新。
3. 修改 `sessionstore.CreateSandboxWithOptions`：只要 workspace snapshot 非 nil、snapshot ID 非空或 workspace ID 非空，就在首次写 metadata 时初始化 `version: 1,status: pending`；没有 workspace 时保持字段缺省。
4. 保持 JSON 向后兼容：旧 metadata 没有新字段时只解码为 nil，不在 Store load 阶段猜测或自动迁移；迁移由 Provisioner 执行。
5. 保持 remove 行为不变，确认 `RemoveSandbox` 删除整个 sandbox root 时会自然删除 metadata、workspace 和未来 staging。
6. 若采用 `golang.org/x/sync/singleflight`，暂不在本阶段引入业务调用，只把依赖调整留到阶段 2，避免无用途的依赖变更。

测试和验证：

- 在 `pkg/model` 增加状态转换 unit tests，覆盖全部合法/非法边。
- 在 `pkg/storage/sessionstore` 增加 unit tests：workspace snapshot、仅 workspace ID、无 workspace、save/load、Store 重建和旧 JSON 缺省字段。
- 运行：

```bash
./scripts/with-go-toolchain.sh go test ./pkg/model ./pkg/storage/sessionstore -count=1
./scripts/with-go-toolchain.sh go test ./pkg/... -run 'Test.*Workspace.*Provision|Test.*Sandbox.*Persistence' -count=1
```

验收标准：

- 新建带 workspace 的 metadata 在任何 runtime/workspace 操作之前已经是 `pending`。
- 旧 metadata 可加载且不会因缺少字段被改写。
- 无 workspace sandbox 的 JSON 形态没有无意义的新字段。
- 本阶段结束时 `go test ./pkg/model ./pkg/storage/sessionstore` 可通过，现有 Session/Run 行为尚未改变。

停止条件：如果新字段必须暴露到 proto 或依赖 SQLite 才能可靠恢复，停止实施并回到 spec 重新确认；不得擅自扩大公开接口。

## 阶段 2：实现一次性 Workspace Provisioner

目标：建立唯一的、并发安全且可重试的 provisioning 编排层；在不接入所有调用方前先通过 package tests 证明状态机和文件事务行为。

依赖：阶段 1 的 metadata 状态和 sessionstore 持久化能力。

实施步骤：

1. 在 `pkg/workspaces` 新增 `Provisioner`，通过窄接口依赖：
   - `GetWorkspaceConfig`。
   - `GetSandbox`、`UpdateSandbox`。
   - application config。
   - 可替换的 provider materializer，便于测试调用次数和故障。
2. 提供 `Ensure(ctx, sandbox)`：
   - 校验 sandbox ID 和 workspace path。
   - 按 sandbox ID 通过进程级 `singleflight.Group` 串行化。
   - 进入共享执行函数后重新从 Store 加载 sandbox。
   - 共享执行结束后再次加载最终 metadata，并同步到每个调用方对象；使用现有 transient-field restore helper 保留 runtime/provider env。
3. 固化分支顺序：
   - 无 workspace 引用：直接成功，不创建状态。
   - 状态 nil 且存在 workspace：按 legacy 迁移直接持久化 `ready`，不解析 config、不读取 workspace 内容。
   - `ready`：直接成功，不调用 config store、materializer 或 staging helper。
   - `pending/failed`：进入首次初始化流程。
   - 未知 version/status：返回错误并保持 workspace 不变。
4. 对 `failed` 重试先合法转换为 `pending` 并持久化；若保存失败，不执行 materialization。
5. 实现 staging helper：
   - staging 位于 `<sandbox>/state/workspace-provisioning/attempt-<id>`。
   - 每次 pending attempt 开始前删除该 sandbox 遗留的 attempt 目录。
   - 克隆 sandbox 对象并仅把 `WorkspacePath` 指向 staging，provider 不得接触正式 workspace。
   - file copy 或 Git clone/checkout 全部成功后，删除仅属于 pending/failed 的正式空目录或半成品目录，再把 staging rename 为正式 workspace。
   - 提升成功后持久化 `ready`；ready 保存失败时返回错误且不得启动 runtime。
6. 失败处理：
   - materializer、staging、提升失败时尝试写入 `failed`。
   - 状态写入也失败时用 `errors.Join` 保留原始错误和持久化错误。
   - 清理 staging 的失败不能覆盖首要业务错误，但必须进入返回错误或可诊断日志。
7. 收紧 provider 边界：把现有直接 session preparation 降为 Provisioner 内部 materialization；移除 Git `HostWorkspaceInitialized` 作为生命周期判断的用途。provider 可以保留纯 config/path helper，但不能再凭正式 workspace 内容跳过 clone。
8. 将 `golang.org/x/sync` 从 indirect 提升为 direct，并在代码完成后运行 `go mod tidy`；不得引入其他新依赖。

测试和验证：

- file unit tests：首次复制一次、ready source 变更不覆盖、config 删除仍成功、修改/删除/重命名/新增/symlink manifest 保持。
- Git unit tests：首次 clone、ready 后远端更新不 fetch、本地修改和清空后不 re-clone。
- legacy tests：running/stopped/pending/failed 和空 workspace 均只写 ready，materializer 调用为零。
- retry tests：materialization 失败、提升失败、ready 保存失败、failed 保存失败、遗留 staging 清理。
- concurrency tests：同一 sandbox 多个 Ensure 只 materialize 一次，不同 sandbox 能并发；所有等待者得到最终持久化对象。
- 运行：

```bash
./scripts/with-go-toolchain.sh go test ./pkg/workspaces ./pkg/model ./pkg/storage/sessionstore -count=1
./scripts/with-go-toolchain.sh go test -race ./pkg/workspaces -count=1
```

验收标准：

- `ready` 分支在测试中可以配合“config store/materializer 被调用即 panic”并稳定通过。
- staging 只处理 pending/failed sandbox；任何测试都不能观察到 ready workspace 被删除、stat、copy 或 clone。
- 首次失败后 runtime 尚未参与，修复 source 后可重试到 ready。
- 同一 sandbox 并发调用没有重复复制、数据竞争或陈旧对象覆盖。

停止条件：如果 staging 与正式 workspace 不能保证位于同一文件系统，或提升需要修改 runtime mount contract，停止实施并重新设计；不得退回在 ready workspace 上原地覆盖的方案。

## 阶段 3：接入所有 sandbox 生命周期入口

目标：生产 service graph 中只有 Provisioner 可以执行 workspace materialization，所有 create/resume 路径共享同一个持久化判断。

依赖：阶段 2 的 Provisioner API 和测试稳定。

实施步骤：

1. 在 `pkg/agentcompose/app` 注册单例 Provisioner，注册顺序位于 session bridge、loader runner 和 run controller 之前。
2. 为调用层定义窄的 `WorkspaceEnsurer` 接口，只暴露 `Ensure`；构造函数和 controller dependencies 注入该接口，测试可替换 fake。
3. v1 Session：
   - create 在 runtime start 前调用 Ensure。
   - `sessions.Lifecycle.ResumeLoaded` 使用 Ensure；ready 时为无文件副作用的 no-op。
   - `EnsureProxyReady` 使用同一 Ensure，禁止保留独立 prepare 分支。
4. Loader：新 sandbox create 与 `LoadOrResume` 注入并调用同一个 Provisioner；running sticky sandbox 的既有快速返回保持不变。
5. Project run：`startProjectRunSandbox` 统一调用 Ensure，新 sandbox 进入 pending 初始化，显式 `sandbox_id` 和 sticky binding 复用 ready workspace；不把本次 run 新解析的 workspace snapshot 覆盖到已有 sandbox。
6. 删除 adapters、sessions 和 runs 对 `workspaces.PrepareSessionWorkspace` 的直接调用。完成后使用 `rg` 确认生产代码中 materializer 只被 Provisioner 引用。
7. 保持现有顺序：workspace ready → capability/runtime/home 受管文件准备 → driver start → VM status running。runtime start 失败只更新 VM status/error，不转换 workspace ready 状态。
8. 保持现有错误映射、event、dashboard notification、token 和 cleanup policy；本阶段不新增 provisioning event 或公开字段。
9. 更新现有 fakes/fixtures 和构造器测试，确保 nil Provisioner 只允许在明确不触发 workspace 的 unit branch；生产 DI 必须始终提供实例。

测试和验证：

- `pkg/sessions`：ResumeLoaded、EnsureProxyReady 成功/失败以及 driver start 前置顺序。
- `pkg/agentcompose/adapters`：v1 create/stop/resume、loader create/sticky resume，使用 fake Ensurer 断言调用次数和错误短路。
- `pkg/runs`：新 sandbox、显式 sandbox、sticky binding、runtime start failure、cleanup policy 非回归。
- 静态检查：

```bash
rg -n 'PrepareSessionWorkspace\(' pkg
rg -n 'HostWorkspaceInitialized\(' pkg
```

生产调用只允许出现在 Provisioner/provider 内部；测试中的直接 materializer 使用必须明确是 provider focused test。

- 运行：

```bash
./scripts/with-go-toolchain.sh go test ./pkg/sessions ./pkg/agentcompose/adapters ./pkg/runs ./pkg/agentcompose/app -count=1
./scripts/with-go-toolchain.sh go test ./pkg/... -count=1
```

验收标准：

- v1、Jupyter、loader、project run 的 create/resume 全部经过同一个 Provisioner 实例。
- Provisioner 返回错误时 driver start 调用次数为零，sandbox 保持现有 failed 错误映射。
- runtime start 失败后 workspace 保持 ready，下一次 resume 不 materialize。
- 不再存在根据 workspace 目录非空决定是否 resume 初始化的生产路径。

停止条件：如果某个生产入口无法注入单例 Provisioner，或必须绕过 Ensure 才能维持现有 API，停止并先修正依赖边界；不得留下双轨 provisioning。

## 阶段 4：补齐跨组件 Integration 回归

目标：使用真实文件/metadata store 和 fake driver 证明状态跨组件、跨 Store 实例和所有复用路径保持，而不仅是 Provisioner 单元逻辑正确。

依赖：阶段 3 完成全部生产接入。

实施步骤：

1. 增加 workspace manifest 测试 helper，递归记录相对路径、类型、mode、内容 hash 和 symlink target；排除 atime 等不稳定字段，不跟随 symlink。
2. 扩展 v1 Session integration：file workspace create 后修改同名文件、删除模板文件、新增普通文件和 symlink；stop、改变模板、resume 后对比 manifest 完全一致。
3. 增加 `EnsureProxyReady` integration，确认自动恢复在 driver start 前没有 workspace 文件操作。
4. 增加 loader sticky integration，连续两次 run 复用同一 sandbox 并保留第一次产生的文件。
5. 增加 project local integration：
   - run A 使用 source v1 并修改 workspace。
   - source 更新为 v2 后复用 run A sandbox，仍保持 A 的修改。
   - 新建 run B sandbox，得到 source v2 且没有 A 的生成文件。
6. 增加 daemon persistence 等价测试：释放 Store/Provisioner，使用相同 DataRoot/SandboxRoot 重建实例，再 resume ready sandbox。
7. 增加 legacy migration integration：直接写入缺少新字段的 metadata，删除关联 config 后 resume，断言只补写 ready 且 workspace manifest 不变。
8. 增加首次失败重试 integration：构造 file/Git source 错误，断言 driver 未启动；修复后重试成功且只启动一次。
9. 为 unit、integration 测试命名遵守 `scripts/run-go-test-shape.sh`：integration 函数名必须包含 `Integration`，不要增加仅为 coverage 重复执行同一 helper 的 E2E wrapper。

测试和验证：

```bash
task test:unit
task test:integration
./scripts/with-go-toolchain.sh go test ./pkg/workspaces ./pkg/sessions ./pkg/agentcompose/adapters ./pkg/runs -count=1
```

验收标准：

- file/local/Git 的关键状态保持和 legacy/失败路径均有最窄 unit test。
- v1、Jupyter、loader、project run 和 Store 重建均有 integration 证明。
- “已有 sandbox 保持、新 sandbox 获取最新 source”在同一测试中被同时断言。
- 测试不依赖真实 Docker、网络、执行顺序或共享目录，可以进入默认 `task test` 门禁。

停止条件：如果 integration 只能通过读取或改写生产 SQLite 来绕过正式控制边界，停止并改用真实 Store/API fixture；正常业务状态不能由数据库故障注入构造。

## 阶段 5：增加真实 Docker E2E 和运维文档

目标：通过编译后的 daemon、正式 API、持久化目录和真实 Docker runtime 证明用户可见的 stop/restart/resume 行为。

依赖：阶段 4 的默认门禁测试稳定；本机有 Docker Engine 和兼容 guest image。

实施步骤：

1. 将 `test/e2e/docker_jupyter_host_daemon_test.go` 中通用的 binary build、daemon process、端口、HTTP client、环境覆盖、日志 buffer 和 Docker cleanup helper 提取到同 package helper；保持现有 Jupyter E2E 行为不变。
2. 新增 `TestE2EDockerFileWorkspaceResumePreservesState`：
   - 通过 v1 ConfigService 创建 file workspace。
   - 通过正式 upload HTTP API 写入 `modified.txt`、`deleted.txt`。
   - 通过 v1 SessionService 创建 Docker sandbox。
   - 通过 v2 ExecService 修改、删除和新增 workspace 文件。
   - stop sandbox，并通过 upload API 更新 source 模板。
   - 停止 daemon，用相同 DataRoot/SandboxRoot 重启。
   - resume 同一 sandbox，断言修改保持、删除未复活、生成文件存在、sandbox/runtime handle 复用。
   - 创建第二个 sandbox，断言它获得更新模板且没有第一个 sandbox 的生成文件。
   - 通过 workspace list/download 验证 sandbox 修改未回写模板。
3. E2E cleanup 必须注册在资源创建后立即注册：公开 RemoveSandbox、Docker label fallback、daemon stop；失败日志包含 daemon output 和资源 ID，不输出 secret env/token。
4. 在 `Taskfile.yml` 新增 `test:e2e:docker-workspace-resume`：依赖 `build:agent-compose`，默认使用本地 `agent-compose-guest:latest`，允许 `AGENT_COMPOSE_E2E_DOCKER_WORKSPACE_IMAGE` 覆盖；镜像不存在时明确失败并提示构建，不隐式 pull。
5. 不修改 `.github/workflows/ci.yml`：真实 Docker E2E 保持 opt-in；默认 CI 继续由 Go tests 和 coverage job 执行阶段 4 的回归。
6. 更新 `TESTING.md`，记录前置条件、task、image override、测试流程和故障诊断。
7. 更新中英文 `docs/design/agent-compose_design.md`：Workspace Source 是一次性 seed、ready resume 保持、new sandbox 获取最新 source、remove 删除。
8. 更新 `docs/spec/core-e2e-test-strategy-spec.md` 中 workspace/stop-resume 场景，引用本次 focused Docker E2E；不要把单 Docker 结果描述为三 driver 等价证明。

测试和验证：

```bash
task test:e2e
task test:e2e:docker-jupyter
task test:e2e:docker-workspace-resume
```

若本机缺少 Jupyter guest 前置条件，至少保证现有 Jupyter test 编译并记录未执行原因；workspace E2E 是本变更的必需验收，不能以 skip 代替通过。

验收标准：

- Docker E2E 经历真实 daemon restart，而不只是同进程 stop/resume。
- 同一 sandbox 的 runtime handle 和 workspace 状态保持，新 sandbox 得到最新 source。
- E2E 完成后没有残留 daemon、容器、socket、端口或临时目录。
- Task/TESTING/设计文档与实际命令、环境变量和行为一致。
- 现有 Docker Jupyter E2E helper 重构后无行为回归。

停止条件：Docker Engine 或指定 guest image 不可用时，将真实 E2E 标为环境阻塞并保留可复现命令；不得在未实际通过时宣称完成。若 E2E 需要新增产品 API 才能执行，停止并重新确认范围，不能加入测试专用后门。

## 阶段 6：全量门禁和交付收口

目标：确认实现、测试、文档和依赖变更满足项目 harness，且没有范围外接口或生成物变化。

依赖：阶段 1 至 5 全部完成；Docker workspace E2E 已真实通过。

实施步骤：

1. 运行 `go mod tidy`，确认只把 `golang.org/x/sync` 提升为 direct 或产生必要的依赖排序变化；任何额外依赖变动都必须解释或撤销。
2. 使用 `rg` 审计所有 workspace preparation、resume 和 constructor call site，确认没有遗漏或测试专用生产分支。
3. 检查 git diff：不应包含 proto/generated client、SQLite migration、compose、环境变量、镜像或公开 CLI 变更。
4. 依次执行 focused tests、真实 Docker E2E、lint、coverage test 和 build；任何失败都回到所属阶段修复，不通过降低断言、skip 或 coverage exclusion 处理。
5. 检查 `task test` 输出，必须继续显示并满足：unit ≥ 60%、integration ≥ 60%、E2E ≥ 60%、combined ≥ 70%。
6. 确认文档使用当前态语气，spec 与 implementation plan 不写入实现完成声明；设计文档在代码完成后反映最终行为。

最终验证命令：

```bash
./scripts/with-go-toolchain.sh go test -race ./pkg/workspaces -count=1
task test:e2e:docker-workspace-resume
task lint
task test
task build
```

验收标准：

- spec 的全部验收标准均能映射到已通过的 unit、integration 或真实 E2E 证据。
- `task lint`、`task test`、`task build` 和 Docker workspace E2E 全部通过。
- coverage baseline 未下降到门禁以下，没有新增静默 exclusion。
- 工作区只包含本变更需要的代码、测试、Taskfile 和文档修改。
- 没有未说明的兼容性、部署或公开接口变化。

## 风险和全局停止条件

- 数据破坏风险：任何代码路径在 `ready` 状态下删除、覆盖、clone、copy 或 staging-promote 正式 workspace，立即停止并修复，不接受“多数情况下保持”的结果。
- 双轨风险：若生产代码仍可绕过 Provisioner 直接 materialize，功能视为未完成。
- 迁移风险：legacy 缺省状态不得按目录内容、VM status 或 config 可用性重新初始化。
- 并发风险：若 race test 显示同一 sandbox 重复 materialization 或内存对象覆盖持久化状态，不进入生命周期接入阶段。
- 错误顺序风险：未持久化 ready 前不得启动 runtime；Provisioner 错误后 driver start 必须为零次。
- E2E 风险：真实 Docker E2E 未执行或仅 skip 时，不满足最终验收。
- 范围风险：实现中若需要 reset API、proto、SQLite、CI runtime 环境或三 driver E2E，先暂停并更新 spec，不能隐式扩张。

## 首版不做的事项

- workspace reset、refresh、reseed 或 force-reinitialize API/CLI。
- sandbox 到 file/local source 的反向同步。
- Git 自动 commit、push、pull、fetch 或 merge。
- project/workspace config 更新已有 sandbox。
- 公开 provisioning 状态。
- SQLite migration、runtime mount 或 volume mount 变更。
- BoxLite/Microsandbox 真实 provisioning E2E。
- 多 daemon 共享同一 SandboxRoot 的分布式锁。
- 与 workspace resume 无关的 sessionstore、run controller 或 E2E harness 重构。
