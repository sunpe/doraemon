# Python MCP Client 设计说明

## 目标

Python MCP client 的目标是提供一个轻量、可直接复制到运维环境中使用的 Doraemon 调用入口。它只依赖 Python 标准库，不引入额外包管理或安装步骤，适合在受限服务器、一次性脚本和 agent 集成中使用。

## 非目标

- 不实现完整 MCP SDK。
- 不维护长连接或流式传输。
- 不保存 token，不读取密钥文件。
- 不做本地权限决策；权限仍由 Doraemon 服务端负责。
- 不把服务端返回的 `stdout`、`stderr` 或审计内容写入本地文件。

## 文件结构

```text
clients/python/doraemon_mcp_client.py
clients/python/test_doraemon_mcp_client.py
```

`doraemon_mcp_client.py` 同时提供可导入 API 和命令行入口。测试使用本地 mock HTTP server 验证请求格式、认证头和错误处理。

## API 设计

核心类是 `DoraemonMCPClient`：

```python
client = DoraemonMCPClient("http://127.0.0.1:8765", "nt_xxx")
tools = client.list_tools()
result = client.call_tool("host.status.get", {})
```

构造参数：

- `base_url`：Doraemon 服务地址，可以是 `http://host:port`，也可以直接传入 `http://host:port/mcp`。
- `token`：Bearer token 明文。调用方负责从安全来源读取。
- `timeout`：HTTP 请求超时时间，默认 30 秒。

方法：

- `list_tools()`：调用 JSON-RPC `tools/list`，返回工具名列表。
- `call_tool(name, arguments)`：调用 JSON-RPC `tools/call`，返回服务端 `result`。

错误：

- JSON-RPC `error` 会转换为 `MCPError`。
- HTTP 错误会转换为 `MCPError`，错误信息包含状态码和响应内容。
- 非 JSON 响应会转换为 `MCPError`。

## 请求格式

客户端固定向 `/mcp` 发送 HTTP POST：

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "host.status.get",
    "arguments": {}
  }
}
```

请求头：

```text
Authorization: Bearer <token>
Content-Type: application/json
```

## 命令行设计

命令行入口支持两个子命令：

```bash
python3 clients/python/doraemon_mcp_client.py list
python3 clients/python/doraemon_mcp_client.py call host.status.get --arguments '{}'
```

公共参数：

- `--url`：服务地址，默认读取 `DORAEMON_URL`，再回退到 `http://127.0.0.1:8765`。
- `--token`：Bearer token，默认读取 `DORAEMON_TOKEN`。
- `--timeout`：请求超时秒数。

输出统一为 JSON，方便 shell、agent 或其他脚本继续处理。

## 安全考虑

客户端不做任何绕过服务端策略的能力。它只负责构造 MCP JSON-RPC 请求，所有鉴权、角色授权、参数策略、路径保护、deny token 和审计写入仍在 Doraemon 服务端执行。

调用方不应把 token 写入命令历史、日志或代码仓库。推荐通过环境变量或外部密钥管理系统注入 `DORAEMON_TOKEN`。

## 测试策略

测试覆盖以下行为：

- `tools/list` 请求使用正确路径、JSON-RPC body 和 Bearer token。
- `tools/call` 请求包含工具名和参数。
- JSON-RPC error 被转换为 `MCPError`。

测试不依赖真实 Doraemon 服务，避免把集成环境和客户端单元行为绑定在一起。
