# Python MCP Client 用户指导

## 前置条件

- Python 3.10 或更高版本。
- 已启动 Doraemon 服务。
- 已创建用户和 token，并把 token 明文保存到安全位置。

启动本地示例服务：

```bash
go run ./cmd/doraemon --config-dir configs/example serve
```

## 配置环境变量

推荐通过环境变量传入服务地址和 token：

```bash
export DORAEMON_URL=http://127.0.0.1:8765
export DORAEMON_TOKEN=nt_xxx
```

不要把真实 token 提交到仓库、写入公开日志或粘贴到不可信系统。

## 列出工具

```bash
python3 clients/python/doraemon_mcp_client.py list
```

输出示例：

```json
{
  "tools": [
    "audit.query",
    "file.list",
    "file.read",
    "host.status.get"
  ]
}
```

实际工具列表由服务端配置和内置工具共同决定。

## 调用工具

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

读取允许范围内的文件：

```bash
python3 clients/python/doraemon_mcp_client.py \
  call file.read \
  --arguments '{"path":"/var/log/app.log"}'
```

如果服务端拒绝请求，客户端会在 stderr 输出错误，并返回非零退出码。

## 在 Python 代码中使用

```python
from clients.python.doraemon_mcp_client import DoraemonMCPClient

client = DoraemonMCPClient("http://127.0.0.1:8765", "nt_xxx")

for tool in client.list_tools():
    print(tool)

result = client.call_tool("host.status.get", {})
print(result)
```

## 常见错误

`--token or DORAEMON_TOKEN is required`

表示没有提供 token。通过 `--token` 参数或 `DORAEMON_TOKEN` 环境变量传入。

`unauthorized`

表示 token 无效、过期、被吊销，或所属用户已禁用。

`tool_not_allowed`

表示当前用户角色没有调用该工具的权限。

`namespace_not_allowed`、`container_not_allowed`、`service_not_allowed`

表示参数没有通过服务端策略。

`path denied`

表示文件路径不在服务端允许读取的根目录内，或符号链接解析后逃逸了允许范围。

## 调试建议

先确认服务健康：

```bash
curl -s http://127.0.0.1:8765/healthz
```

再确认 token 是否可用：

```bash
python3 clients/python/doraemon_mcp_client.py list
```

最后查看审计：

```bash
go run ./cmd/doraemon --config-dir configs/example audit list --since 1h
```
