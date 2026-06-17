#!/usr/bin/env python3
"""Small standard-library MCP client for Doraemon."""

from __future__ import annotations

import argparse
import json
import os
import sys
import urllib.error
import urllib.request
from typing import Any


class MCPError(RuntimeError):
    """Raised when the MCP endpoint returns a JSON-RPC or HTTP error."""


class DoraemonMCPClient:
    def __init__(self, base_url: str, token: str, timeout: float = 30.0) -> None:
        if not base_url:
            raise ValueError("base_url is required")
        if not token:
            raise ValueError("token is required")
        self.endpoint = self._endpoint(base_url)
        self.token = token
        self.timeout = timeout
        self._next_id = 1

    def list_tools(self) -> list[str]:
        result = self._request("tools/list", {})
        tools = result.get("tools", [])
        if not isinstance(tools, list):
            raise MCPError("invalid tools/list response")
        return tools

    def call_tool(self, name: str, arguments: dict[str, Any] | None = None) -> Any:
        if not name:
            raise ValueError("tool name is required")
        return self._request(
            "tools/call",
            {"name": name, "arguments": arguments or {}},
        )

    def _request(self, method: str, params: dict[str, Any]) -> Any:
        request_id = self._next_id
        self._next_id += 1
        payload = {
            "jsonrpc": "2.0",
            "id": request_id,
            "method": method,
            "params": params,
        }
        body = json.dumps(payload).encode("utf-8")
        request = urllib.request.Request(
            self.endpoint,
            data=body,
            headers={
                "Authorization": f"Bearer {self.token}",
                "Content-Type": "application/json",
            },
            method="POST",
        )
        try:
            with urllib.request.urlopen(request, timeout=self.timeout) as response:
                raw = response.read()
        except urllib.error.HTTPError as exc:
            detail = exc.read().decode("utf-8", errors="replace")
            raise MCPError(f"HTTP {exc.code}: {detail}") from exc
        except urllib.error.URLError as exc:
            raise MCPError(str(exc.reason)) from exc

        try:
            decoded = json.loads(raw.decode("utf-8"))
        except json.JSONDecodeError as exc:
            raise MCPError("invalid JSON response") from exc
        if "error" in decoded and decoded["error"] is not None:
            error = decoded["error"]
            message = error.get("message", "json-rpc error") if isinstance(error, dict) else str(error)
            raise MCPError(message)
        return decoded.get("result")

    @staticmethod
    def _endpoint(base_url: str) -> str:
        trimmed = base_url.rstrip("/")
        if trimmed.endswith("/mcp"):
            return trimmed
        return trimmed + "/mcp"


def _parse_json_object(raw: str) -> dict[str, Any]:
    if raw == "":
        return {}
    value = json.loads(raw)
    if not isinstance(value, dict):
        raise ValueError("arguments must be a JSON object")
    return value


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Doraemon MCP HTTP client")
    parser.add_argument("--url", default=os.getenv("DORAEMON_URL", "http://127.0.0.1:8765"))
    parser.add_argument("--token", default=os.getenv("DORAEMON_TOKEN"))
    parser.add_argument("--timeout", type=float, default=30.0)

    subparsers = parser.add_subparsers(dest="command", required=True)
    subparsers.add_parser("list", help="list available MCP tools")

    call = subparsers.add_parser("call", help="call an MCP tool")
    call.add_argument("name", help="tool name, for example host.status.get")
    call.add_argument(
        "--arguments",
        default="{}",
        help='tool arguments as a JSON object, for example \'{"namespace":"default"}\'',
    )

    args = parser.parse_args(argv)
    if not args.token:
        parser.error("--token or DORAEMON_TOKEN is required")

    client = DoraemonMCPClient(args.url, args.token, args.timeout)
    try:
        if args.command == "list":
            result = {"tools": client.list_tools()}
        else:
            result = client.call_tool(args.name, _parse_json_object(args.arguments))
    except (json.JSONDecodeError, ValueError, MCPError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1

    print(json.dumps(result, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
