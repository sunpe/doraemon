#!/usr/bin/env python3
"""Small MCP SDK client for Doraemon."""

from __future__ import annotations

import argparse
import asyncio
import json
import os
import sys
from typing import Any

import httpx
from mcp import ClientSession
from mcp.client.streamable_http import streamablehttp_client


class MCPError(RuntimeError):
    """Raised when the MCP endpoint returns a JSON-RPC or HTTP error."""


class DoraemonMCPClient:
    def __init__(
        self,
        base_url: str,
        token: str,
        timeout: float = 30.0,
        trust_env: bool = False,
    ) -> None:
        if not base_url:
            raise ValueError("base_url is required")
        if not token:
            raise ValueError("token is required")
        self.endpoint = self._endpoint(base_url)
        self.token = token
        self.timeout = timeout
        self.trust_env = trust_env

    def list_tools(self) -> list[str]:
        return asyncio.run(self._list_tools())

    async def _list_tools(self) -> list[str]:
        result = await self._with_session(lambda session: session.list_tools())
        names = []
        for tool in result.tools:
            name = getattr(tool, "name", None)
            if not isinstance(name, str):
                raise MCPError("invalid tool entry")
            names.append(name)
        return names

    def call_tool(self, name: str, arguments: dict[str, Any] | None = None) -> Any:
        if not name:
            raise ValueError("tool name is required")
        return asyncio.run(self._call_tool(name, arguments or {}))

    async def _call_tool(self, name: str, arguments: dict[str, Any]) -> Any:
        result = await self._with_session(lambda session: session.call_tool(name, arguments))
        if getattr(result, "isError", False):
            raise MCPError(_content_text(result.content) or "tool error")
        structured = getattr(result, "structuredContent", None)
        if structured is not None:
            return structured
        return {"content": [_content_to_json(content) for content in result.content]}

    async def _with_session(self, operation):
        headers = {"Authorization": f"Bearer {self.token}"}
        try:
            async with streamablehttp_client(
                self.endpoint,
                headers=headers,
                timeout=self.timeout,
                httpx_client_factory=self._httpx_client,
            ) as (read_stream, write_stream, _):
                async with ClientSession(read_stream, write_stream) as session:
                    await session.initialize()
                    return await operation(session)
        except MCPError:
            raise
        except Exception as exc:
            raise MCPError(str(exc)) from exc

    def _httpx_client(
        self,
        headers: dict[str, str] | None = None,
        timeout: httpx.Timeout | None = None,
        auth: httpx.Auth | None = None,
    ) -> httpx.AsyncClient:
        return httpx.AsyncClient(
            follow_redirects=True,
            headers=headers,
            timeout=timeout,
            auth=auth,
            trust_env=self.trust_env,
        )

    @staticmethod
    def _endpoint(base_url: str) -> str:
        trimmed = base_url.rstrip("/")
        if trimmed.endswith("/mcp"):
            return trimmed
        return trimmed + "/mcp"


def _content_text(content: list[Any]) -> str:
    texts = []
    for item in content:
        text = getattr(item, "text", None)
        if isinstance(text, str):
            texts.append(text)
    return "\n".join(texts)


def _content_to_json(content: Any) -> Any:
    if hasattr(content, "model_dump"):
        return content.model_dump(by_alias=True, exclude_none=True)
    if hasattr(content, "__dict__"):
        return content.__dict__
    return content


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
    parser.add_argument(
        "--trust-env",
        action="store_true",
        help="allow httpx to use proxy settings from the environment or system",
    )

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

    client = DoraemonMCPClient(args.url, args.token, args.timeout, trust_env=args.trust_env)
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
