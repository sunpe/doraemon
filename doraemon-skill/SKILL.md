---
name: doraemon-ops
description: Use when operating or troubleshooting remote servers through Doraemon MCP, including host checks, Docker status, Kubernetes inspection, file reads, audit queries, MCP connection setup, and policy-denial explanations. Trigger when the user asks an agent to inspect, diagnose, or operate a remote machine via Doraemon instead of SSH or arbitrary shell.
---

# Doraemon Ops

Use Doraemon MCP as the controlled operations boundary. Do not bypass Doraemon with SSH, arbitrary shell, or direct host access unless the user explicitly asks for a separate non-Doraemon path.

Never print, persist, commit, or echo bearer tokens. Prefer environment variables or the agent MCP configuration.

## Connection

Expect a configured MCP server such as `doraemon_prod` or `doraemon_10_24_160_120`.

For remote Streamable HTTP MCP, the endpoint shape is:

```text
http://host:port/mcp
```

Some deployments run plain HTTP on port `443`; do not assume HTTPS from the port alone. If connection fails with TLS errors such as `WRONG_VERSION_NUMBER`, retry or ask whether the endpoint is plain HTTP.

If no Doraemon MCP server is exposed in the current agent session, say that clearly. A skill explains how to operate Doraemon, but the agent must still have an MCP server configured before it can call Doraemon tools.

## Default Workflow

1. Call `tools/list` or inspect the available MCP tools.
2. Call `host.status.get` to confirm the target host and basic runtime.
3. Pick the narrowest read-only tool for the question.
4. Summarize findings, denied actions, and the next safe checks.
5. Do not retry denied operations with broader or invented parameters.

For common tool choices, read [references/tools.md](references/tools.md).

For connection failures, authorization failures, or policy denials, read [references/troubleshooting.md](references/troubleshooting.md).

## Safe Operating Style

- Start with overview tools before detailed tools.
- Prefer read-only checks: status, list, describe, logs, audit.
- Keep arguments narrow: one namespace, one pod, one container, one path.
- Treat all denials as policy signals, not obstacles to bypass.
- Include exact denial reasons in the final answer when useful.
- Separate confirmed facts from likely causes.

## Common First Checks

For a host overview:

```text
host.status.get
```

For Docker health:

```text
docker.system.info
docker.containers.list
docker.containers.all.list
```

For Kubernetes health:

```text
k8s.namespaces.list
k8s.pods.list
k8s.events.list
```

For audit context:

```text
audit.query
```

## Reporting

When reporting results, include:

- target server or configured MCP server name
- tools successfully called
- important state counts, such as running/restarting/stopped containers
- denied calls and denial reasons
- concrete next checks that stay within Doraemon policy
