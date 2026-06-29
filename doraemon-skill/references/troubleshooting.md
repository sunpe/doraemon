# Doraemon Troubleshooting

## MCP Server Not Visible

If the current agent session has no Doraemon MCP server or tools, explain that the server must be configured before the skill can call it.

Remote Streamable HTTP configuration usually needs:

```json
{
  "mcpServers": {
    "doraemon_prod": {
      "type": "streamable-http",
      "url": "http://host:port/mcp",
      "headers": {
        "Authorization": "Bearer ${DORAEMON_TOKEN}"
      }
    }
  }
}
```

Do not put real tokens in files committed to a repository.

## HTTP Versus HTTPS

Do not infer TLS from port `443`.

- `SSL: WRONG_VERSION_NUMBER`: often means the endpoint is plain HTTP, not HTTPS.
- `502 Bad Gateway`: may indicate a proxy, gateway, or upstream routing issue.
- `401 unauthorized`: token missing, invalid, expired, revoked, or user disabled.

For Python/httpx-based clients, system proxy settings can affect internal IPs. Use direct mode unless the user explicitly needs a proxy.

## Policy Denials

Treat these as expected Doraemon controls:

- `tool_not_allowed`: the user's role lacks the tool.
- `container_not_allowed`: Docker policy blocks the container name or ID.
- `namespace_not_allowed`: Kubernetes policy blocks the namespace.
- `service_not_allowed`: host service policy blocks the service.
- `path denied`: file path is outside allowed roots or resolves outside allowed roots.
- `deny_token`: command or argument matched a configured deny rule.
- `high_risk_not_allowed`: high-risk tool lacks a matching unexpired allow rule.

Do not try alternative spellings or broader parameters to bypass policy. Ask for role or policy changes if the user needs that access.

## Reporting Failures

Report:

- which tool was attempted
- the exact denial or transport error
- what was confirmed before the failure
- the smallest safe next step

Example:

```text
docker.container.inspect zxy-scorm was denied with container_not_allowed.
The token can list containers, and the container is restarting, but this token cannot inspect that container.
```
