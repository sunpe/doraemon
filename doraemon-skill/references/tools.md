# Doraemon Tool Guide

Use the available tool list from the MCP server as the source of truth. Tool availability depends on the Doraemon server configuration, token user, role, and policy.

## Host

- `host.status.get`: basic host identity, OS, architecture, CPU, and runtime status.
- `host.disk.list`: disk overview when available.
- `host.process.list`: process overview when available.
- `host.service.status`: service status for allowed service names.

## Docker

Start with:

```text
docker.system.info
docker.containers.list
docker.containers.all.list
```

Use `docker.containers.list` for running containers. Use `docker.containers.all.list` when the user asks for stopped, restarting, or unhealthy containers.

For a specific allowed container:

```text
docker.container.inspect
docker.container.logs
docker.container.stats
docker.container.top
docker.container.port
```

If `docker.container.inspect` or `docker.container.logs` returns `container_not_allowed`, report that the token policy blocks that container argument.

`docker.system.df` may be slower than overview commands. If it returns a timeout-like failure with no output, report that the disk-usage command did not complete and continue with other safe signals.

## Kubernetes

Start with:

```text
k8s.namespaces.list
k8s.pods.list
k8s.events.list
k8s.nodes.list
```

For a specific workload or pod:

```text
k8s.pod.describe
k8s.pod.logs
k8s.pod.logs.previous
k8s.rollout.status
```

Keep namespace arguments explicit. If a namespace is rejected, report `namespace_not_allowed`.

## Files

Use file tools only for user-requested paths and only within allowed read roots:

```text
file.list
file.read
```

If a path is denied, report the denial and do not attempt path traversal or symlink bypasses.

## Audit

Use audit tools to explain recent denials or actions:

```text
audit.query
```

Prefer a bounded time window when supported, such as recent minutes or hours.
