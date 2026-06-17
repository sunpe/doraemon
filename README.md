# Doraemon

Doraemon 是一个面向 agent 的极简 MCP 运维网关。它运行在目标服务器上，通过受控的 Kubernetes、Docker、主机、文件和审计工具提供运维能力，不暴露 SSH，也不提供通用 shell 执行入口。

项目默认采取保守设计：一个 MCP tool 只做一件事；所有外部命令都使用固定 argv 模板；请求必须经过认证、策略检查、路径保护和审计记录。

## 功能

- 通过 HTTP 暴露 MCP `tools/list` 和 `tools/call`
- Go + Cobra CLI
- TOML 配置
- 使用 `.d` 目录拆分命令配置和规则配置
- bbolt 嵌入式存储
- 按用户管理 token 认证
- 支持一个用户多个 token
- 支持 token 过期和吊销
- 使用 Argon2id 哈希 token，明文只显示一次
- 固定命令模板，不提供通用命令执行器
- 只使用 `exec.CommandContext`，不执行 shell
- 拒绝 shell 可执行文件和命令组合 token
- 针对 Kubernetes、Docker、主机服务工具做参数级策略检查
- 只读路径保护，防止符号链接逃逸
- 高风险工具必须配置带过期时间的临时允许规则
- 对允许、拒绝、认证失败和执行失败写入审计记录
- 提供标准库实现的 Python MCP client

## 为什么需要 Doraemon？

Agent 需要可用的运维工具，但生产服务器不应该暴露 SSH 或任意 shell 执行能力。Doraemon 给 agent 一个小而受控的工具包：每项能力都是显式、可审计、受策略约束的。

## 当前状态

这是 MVP，适合本地测试和设计迭代，还不是已经加固完成的生产版本。

已实现：

- 配置加载和校验
- Cobra CLI
- MCP HTTP endpoint
- 用户和 token 生命周期
- 嵌入式审计存储
- 命令模板执行
- 内置主机、文件和审计工具
- Python MCP client
- 核心策略和路径保护测试

## 架构

```text
Agent
  -> MCP over HTTP
    -> Bearer token auth
      -> user and role resolution
        -> tool authorization
          -> high-risk allow check if needed
            -> input validation
              -> path guard if needed
                -> fixed argv rendering
                  -> deny scan
                    -> exec.CommandContext
                      -> audit write
```

## 安全模型

Doraemon 不提供 `run_command` 工具。Agent 不能提交任意命令字符串，也不能提交完整 argv 数组。

外部命令工具通过固定模板配置：

```toml
[tools."k8s.pods.list"]
executor = "kubectl"
argv = ["get", "pods", "-n", "{{namespace}}", "-o", "json"]
```

用户输入只能填充声明过的模板变量，并且会先经过校验。

设计上禁止：

- `sh`
- `bash`
- `zsh`
- `fish`
- `cmd`
- `powershell`
- `sh -c`
- `bash -c`
- 管道
- 重定向
- 命令替换
- `;`
- `&&`
- `||`
- 反引号
- `$()`

## 配置

Doraemon 使用一个系统配置文件，以及模块化的命令和规则目录：

```text
/etc/doraemon/
  system.toml

  commands.toml
  commands.d/
    10-kubectl.toml
    20-docker.toml
    30-host.toml

  rules.toml
  rules.d/
    10-roles.toml
    20-policy.toml
    40-paths.toml
    90-deny.toml
```

加载规则：

- `system.toml` 只按单文件加载。
- 不加载 `system.d`。
- 先加载 `commands.toml`，再按文件名顺序加载 `commands.d/*.toml`。
- 先加载 `rules.toml`，再按文件名顺序加载 `rules.d/*.toml`。
- 重复的 executor、tool、role、deny rule 或 policy 字段会导致配置校验失败。
- 不支持覆盖或替换行为。

可参考 [`configs/example`](configs/example) 中的示例配置。

## 快速开始

校验示例配置：

```bash
go run ./cmd/doraemon --config-dir configs/example check-config
```

创建用户：

```bash
go run ./cmd/doraemon --config-dir configs/example user create agent-prod --roles readonly
```

创建 token：

```bash
go run ./cmd/doraemon --config-dir configs/example token create \
  --user agent-prod \
  --name local-test \
  --ttl 24h
```

token 明文只会打印一次，请妥善保存。

启动 MCP HTTP 服务：

```bash
go run ./cmd/doraemon --config-dir configs/example serve
```

列出 MCP 工具：

```bash
curl -s http://127.0.0.1:8765/mcp \
  -H "Authorization: Bearer $DORAEMON_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'
```

调用工具：

```bash
curl -s http://127.0.0.1:8765/mcp \
  -H "Authorization: Bearer $DORAEMON_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/call",
    "params": {
      "name": "k8s.pods.list",
      "arguments": {
        "namespace": "default"
      }
    }
  }'
```

## Client

仓库在 [`clients/`](clients/) 目录下维护 MCP client。当前提供标准库实现的 Python client；用法、设计和排障说明见 [clients/README.md](clients/README.md)。

## CLI

```text
doraemon serve --config-dir /etc/doraemon
doraemon check-config --config-dir /etc/doraemon
doraemon config dump --config-dir /etc/doraemon
doraemon user create/list/disable
doraemon token create/list/revoke/rotate
doraemon audit list --since 1h
doraemon policy test --tool k8s.pods.list --input input.json
doraemon version
```

## 内置工具

内置工具不会执行外部命令模板：

- `host.status.get`
- `host.disk.list`
- `host.process.list`
- `file.read`
- `file.list`
- `audit.query`

示例配置中的命令工具：

- `k8s.pods.list`
- `k8s.pods.top`
- `k8s.pod.describe`
- `k8s.pod.logs`
- `k8s.pod.logs.previous`
- `k8s.nodes.list`
- `k8s.nodes.top`
- `k8s.node.describe`
- `k8s.namespaces.list`
- `k8s.events.list`
- `k8s.deployments.list`
- `k8s.rollout.status`
- `k8s.statefulsets.list`
- `k8s.daemonsets.list`
- `k8s.replicasets.list`
- `k8s.services.list`
- `k8s.service.describe`
- `k8s.endpoints.list`
- `k8s.ingresses.list`
- `k8s.configmaps.list`
- `k8s.jobs.list`
- `k8s.cronjobs.list`
- `k8s.persistentvolumeclaims.list`
- `k8s.persistentvolumes.list`
- `docker.containers.list`
- `docker.containers.all.list`
- `docker.container.inspect`
- `docker.container.logs`
- `docker.container.stats`
- `docker.container.top`
- `docker.container.port`
- `docker.images.list`
- `docker.networks.list`
- `docker.network.inspect`
- `docker.volumes.list`
- `docker.volume.inspect`
- `docker.system.info`
- `docker.system.df`
- `docker.version`
- `host.service.status`

## 高风险临时允许

高风险工具必须同时满足角色权限和未过期的临时允许规则：

```toml
[[high_risk.allow]]
name = "restart-nginx-temporary"
tool = "host.service.restart"
users = ["alice"]
token_ids = ["tok_example"]
expires_at = "2026-06-17T23:00:00+08:00"

[high_risk.allow.params]
service = "nginx"
```

临时允许不能绕过 shell 限制、路径限制或固定命令模板。

## 开发

运行 Go 测试：

```bash
go test ./...
```

运行 Python client 测试：

```bash
python3 -m unittest clients/python/test_doraemon_mcp_client.py
```

构建：

```bash
go build ./cmd/doraemon
```

校验示例配置：

```bash
go run ./cmd/doraemon --config-dir configs/example check-config
```

仓库包含 vendored Go 依赖，常规构建会自动使用 `vendor/`。

## 许可证

暂未选择许可证。
