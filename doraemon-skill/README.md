# Doraemon Ops Skill

这个目录是 Doraemon 的通用运维 skill，用于指导 agent 通过 Doraemon MCP 安全地检查和诊断远程服务器。

skill 只提供操作流程和排障知识，不能替代 MCP server 配置。agent 仍然需要在启动或会话配置中注册 Doraemon MCP server，才能真正调用工具。

## 目录结构

```text
doraemon-skill/
  SKILL.md
  agents/
    openai.yaml
  references/
    tools.md
    troubleshooting.md
```

- `SKILL.md`：核心工作流，包括连接方式、安全边界、默认检查流程和汇报要求。
- `references/tools.md`：常用 host、Docker、Kubernetes、文件和审计工具说明。
- `references/troubleshooting.md`：MCP 连接、HTTP/HTTPS、代理、鉴权和策略拒绝排障说明。
- `agents/openai.yaml`：skill 展示元信息。

## 使用方式

把 `doraemon-skill` 目录复制或安装到 agent 可发现的 skills 目录中。不同 agent 的目录可能不同，常见位置类似：

```text
~/.codex/skills/doraemon-ops
```

如果 agent 支持从项目目录加载 skill，也可以直接引用本目录。

## MCP server 配置

agent 需要单独配置 Doraemon MCP server，例如：

```json
{
  "mcpServers": {
    "doraemon_mcp": {
      "type": "streamable-http",
      "url": "http://192.0.0.1:443/mcp", // 替换为真实ip
      "headers": {
        "Authorization": "Bearer ${DORAEMON_TOKEN}"
      }
    }
  }
}
```

token 建议通过环境变量注入：

```bash
export DORAEMON_TOKEN="nt_xxx"
```

不要把真实 token 写入 skill、仓库、日志或示例配置。

## 验证

agent 加载 skill 并配置 MCP server 后，先让 agent 执行最小只读检查：

```text
tools/list
host.status.get
docker.system.info
```

如果这些工具可用，说明 MCP 连接、token 认证和基础授权正常。

## 注意事项

- Doraemon 可能是 HTTP 运行在 443 端口，不要只根据端口假设是 HTTPS。
- 如果当前 agent 会话没有暴露 Doraemon MCP server，skill 只能提供指导，不能直接调用工具。
- `tool_not_allowed`、`container_not_allowed`、`namespace_not_allowed` 等返回值是服务端策略控制，不应绕过。
- 运维动作应优先使用只读工具，例如 status、list、describe、logs、audit。
