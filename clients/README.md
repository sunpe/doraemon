# Doraemon Clients

本目录存放 Doraemon MCP client。client 只负责构造 MCP JSON-RPC 请求并携带 Bearer token；认证、授权、参数策略、路径保护、deny token 和审计仍由 Doraemon 服务端执行。

## 当前 client

- Python 标准库 client：[python/doraemon_mcp_client.py](python/doraemon_mcp_client.py)
- Python client 设计说明：[python/python-mcp-client-design.md](python/python-mcp-client-design.md)
- Python client 用户指导：[python/python-mcp-client-guide.md](python/python-mcp-client-guide.md)

## Python 快速使用

通过命令行列出工具：

```bash
python3 clients/python/doraemon_mcp_client.py \
  --url http://127.0.0.1:8765 \
  --token "$DORAEMON_TOKEN" \
  list
```

调用工具：

```bash
python3 clients/python/doraemon_mcp_client.py \
  --url http://127.0.0.1:8765 \
  --token "$DORAEMON_TOKEN" \
  call k8s.pods.list \
  --arguments '{"namespace":"default"}'
```

也可以通过环境变量传入地址和 token：

```bash
export DORAEMON_URL=http://127.0.0.1:8765
export DORAEMON_TOKEN=nt_xxx
python3 clients/python/doraemon_mcp_client.py list
```

## 安全提醒

- 不要把真实 token 提交到仓库。
- 不要把真实 token 写入公开日志或命令示例。
- 推荐通过环境变量或外部密钥管理系统注入 `DORAEMON_TOKEN`。
- client 不能绕过服务端权限和审计；权限问题应从服务端角色、策略和审计记录排查。
