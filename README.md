# Doraemon

Doraemon 是一个面向 agent 的 MCP 运维网关。它运行在目标服务器上，通过受控工具暴露 Kubernetes、Docker、主机、文件和审计能力，不提供 SSH，也不提供通用 shell runner。

核心原则：

- 一个 MCP tool 只做一件事
- 外部命令使用固定 argv 模板
- 所有请求经过 Bearer token 认证、角色授权、策略检查、路径保护和审计
- 不允许 agent 提交任意命令字符串或完整 argv

## 状态

当前是 MVP，适合测试、内网试用和设计迭代，还不是完整加固后的生产版本。

已包含：

- MCP HTTP endpoint：`tools/list`、`tools/call`
- Go + Cobra CLI
- TOML 配置和 `.d` 目录加载
- bbolt 用户、token 和审计存储
- Argon2id token 哈希，token 明文只显示一次
- 固定命令模板和 deny 规则
- 只读 path guard，防止符号链接逃逸
- 高风险工具临时 allow 机制
- Python MCP client：[clients/](clients/)

## 快速开始

校验示例配置：

```bash
go run ./cmd/doraemon --config-dir configs/example check-config
```

创建用户和 token：

```bash
go run ./cmd/doraemon --config-dir configs/example user create agent-prod --roles readonly
go run ./cmd/doraemon --config-dir configs/example token create \
  --user agent-prod \
  --name local-test \
  --ttl 24h
```

启动服务：

```bash
go run ./cmd/doraemon --config-dir configs/example serve
```

列出 MCP 工具：

```bash
export DORAEMON_TOKEN=nt_xxx
python3 clients/python/doraemon_mcp_client.py list
```

## 配置

仓库内提供两套配置：

- [`configs/default`](configs/default)：默认部署配置，监听 `0.0.0.0:8765`，存储路径为 `/var/lib/doraemon/doraemon.db`
- [`configs/example`](configs/example)：本地示例配置，规则与 default 保持一致，存储路径为配置目录下的 `./data/doraemon.db`

配置结构：

```text
system.toml
commands.d/
  10-kubectl.toml
  20-docker.toml
  30-host.toml
rules.d/
  10-roles.toml
  20-policy.toml
  40-paths.toml
  90-deny.toml
```

加载规则：

- `system.toml` 只按单文件加载
- `commands.toml` 先于 `commands.d/*.toml`
- `rules.toml` 先于 `rules.d/*.toml`
- `.d` 文件按文件名排序加载
- 重复 executor、tool、role、deny rule 或 policy 字段会校验失败

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

常用参数：

| 命令 | 参数 | 说明 |
| --- | --- | --- |
| 所有 `doraemon` 命令 | `--config-dir <dir>` | 配置目录，默认 `/etc/doraemon`。相对存储路径会以该目录为基准解析。 |
| `config dump` | `--format json\|toml` | 输出生效后的配置，默认 `json`。 |
| `user create <name>` | `--roles <roles>` | 用户角色列表，多个角色用逗号分隔，例如 `readonly,ops`。 |
| `token create` | `--user <name>` | token 所属用户，必填。 |
| `token create` | `--name <name>` | token 名称，必填，用于审计和识别。 |
| `token create` / `token rotate` | `--ttl <duration>` | token 有效期，必填，例如 `24h`、`720h`。 |
| `token list` | `--user <name>` | 只列出指定用户的 token；为空时列出全部。 |
| `token revoke <token-id>` | `<token-id>` | 要吊销的 token ID。 |
| `token rotate <token-id>` | `<token-id>` | 先吊销旧 token，再为同一用户创建替代 token。 |
| `audit list` | `--since <duration>` | 只查看最近一段时间的审计，例如 `1h`、`24h`；最多返回 100 条。 |
| `policy test` | `--tool <name>` | 要检查的已配置命令工具名。 |
| `policy test` | `--input <file>` | 参数 JSON 文件，内容应是对象，例如 `{"namespace":"default"}`。 |

日志配置：

| 配置 | 说明 |
| --- | --- |
| `[logging] command_execution = true` | 外部命令执行完成后打印命令、参数、耗时、退出码和 stdout/stderr 字节数；默认关闭，不打印命令输出正文。 |

## Client

Python client 位于 [clients/python](clients/python)，基于官方 MCP Python SDK：

```bash
python3 clients/python/doraemon_mcp_client.py list
python3 clients/python/doraemon_mcp_client.py call host.status.get
```

Python client 常用参数：

| 参数 | 说明 |
| --- | --- |
| `--url <url>` | Doraemon 服务地址，默认读取 `DORAEMON_URL`，再回退到 `http://127.0.0.1:8765`。可以传 `/mcp`，也可以只传主机和端口。 |
| `--token <token>` | Bearer token，默认读取 `DORAEMON_TOKEN`。 |
| `--timeout <seconds>` | 请求超时秒数，默认 `30`。 |
| `--trust-env` | 允许 Python/httpx 使用系统或环境代理配置；默认关闭，避免内网地址误走代理。 |
| `call <tool>` | 调用指定 MCP tool，例如 `host.status.get`。 |
| `--arguments <json>` | 工具参数 JSON 对象，默认 `{}`。 |

详细说明见 [clients/README.md](clients/README.md)。

## 构建

本机编译：

```bash
make build
```

编译 Linux 多架构产物：

```bash
make linux
```

产物输出到 `dist/`：

- `doraemon-linux-amd64`
- `doraemon-linux-arm64`
- `doraemon-linux-armv7`

## 开发验证

```bash
go test ./...
/Users/sunpeng/go/bin/golangci-lint run ./...
python3 -m unittest clients/python/test_doraemon_mcp_client.py
go run ./cmd/doraemon --config-dir configs/example check-config
go run ./cmd/doraemon --config-dir configs/default check-config
```

## 安全提醒

`0.0.0.0:8765` 会监听所有网卡。部署时请配合防火墙、安全组、反向代理或内网访问控制，只允许可信来源访问。

不要把 token 写入仓库、日志或公开脚本。高风险工具即使配置临时 allow，也不能绕过 shell 限制、路径限制或固定命令模板。

## License

MIT，见 [LICENSE](LICENSE)。
