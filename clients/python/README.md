# Doraemon Python MCP Client

这是 Doraemon 的 Python MCP client，基于官方 `modelcontextprotocol/python-sdk`，通过 MCP Streamable HTTP transport 连接 Doraemon `/mcp` endpoint。

client 只负责发起 MCP 请求并携带 Bearer token。认证、授权、参数策略、路径保护、deny token 和审计都由 Doraemon 服务端完成。

## 文件

```text
doraemon_mcp_client.py          # 可导入 API 和命令行入口
requirements.txt                # Python 依赖
test_doraemon_mcp_client.py     # 单元测试
```

## 安装

```bash
python3 -m pip install -r clients/python/requirements.txt
```

需要 Python 3.10 或更高版本。

## 配置

推荐通过环境变量传入服务地址和 token：

```bash
export DORAEMON_URL=http://127.0.0.1:8765
export DORAEMON_TOKEN=nt_xxx
```

不要把真实 token 提交到仓库、写入公开日志或粘贴到不可信系统。

## 命令行使用

列出工具：

```bash
python3 clients/python/doraemon_mcp_client.py list
```

调用无参数工具：

```bash
python3 clients/python/doraemon_mcp_client.py call host.status.get
```

调用带参数工具：

```bash
python3 clients/python/doraemon_mcp_client.py \
  call k8s.pods.list \
  --arguments '{"namespace":"default"}'
```

也可以显式传入地址和 token：

```bash
python3 clients/python/doraemon_mcp_client.py \
  --url http://127.0.0.1:8765 \
  --token "$DORAEMON_TOKEN" \
  list
```

输出统一为 JSON，方便 shell、agent 或其他脚本继续处理。

默认不会读取系统代理或环境代理配置，适合直连内网地址。如果确实需要通过代理访问，可以加 `--trust-env`；使用 SOCKS 代理时，需要额外安装 `httpx[socks]`。

## Python API

```python
from clients.python.doraemon_mcp_client import DoraemonMCPClient

client = DoraemonMCPClient("http://127.0.0.1:8765", "nt_xxx")

tools = client.list_tools()
result = client.call_tool("host.status.get", {})
```

`base_url` 可以是 `http://host:port`，也可以直接传入 `http://host:port/mcp`。`timeout` 默认 30 秒，`trust_env` 默认 `False`。

## 常见错误

- `--token or DORAEMON_TOKEN is required`：没有提供 token。
- `unauthorized`：token 无效、过期、被吊销，或所属用户已禁用。
- `tool_not_allowed`：当前用户角色没有调用该工具的权限。
- `namespace_not_allowed` / `container_not_allowed` / `service_not_allowed`：参数没有通过服务端策略。
- `path denied`：文件路径不在服务端允许读取的根目录内，或符号链接解析后逃逸。
- `Using SOCKS proxy, but the 'socksio' package is not installed`：启用了代理环境但缺少 SOCKS 支持。默认直连不会触发；如需代理，请安装 `httpx[socks]` 并使用 `--trust-env`。

## 测试

```bash
python3 -m pytest clients/python
```

测试会 mock 官方 SDK transport 和 `ClientSession`，不依赖真实 Doraemon 服务。
