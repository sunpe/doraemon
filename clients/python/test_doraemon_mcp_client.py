import unittest
from unittest.mock import patch

from clients.python import doraemon_mcp_client
from clients.python.doraemon_mcp_client import DoraemonMCPClient, MCPError


class FakeTransport:
    calls = []

    def __init__(self, url, headers=None, timeout=30, httpx_client_factory=None):
        self.url = url
        self.headers = headers or {}
        self.timeout = timeout
        self.httpx_client_factory = httpx_client_factory

    async def __aenter__(self):
        self.__class__.calls.append(
            {
                "url": self.url,
                "headers": self.headers,
                "timeout": self.timeout,
                "httpx_client_factory": self.httpx_client_factory,
            }
        )
        return "read-stream", "write-stream", lambda: None

    async def __aexit__(self, exc_type, exc, tb):
        return False


class FakeTool:
    def __init__(self, name):
        self.name = name


class FakeListResult:
    def __init__(self):
        self.tools = [FakeTool("file.read"), FakeTool("host.status.get")]


class FakeCallResult:
    structuredContent = {"content": {"hostname": "demo"}}
    isError = False
    content = []


class FakeErrorResult:
    structuredContent = None
    isError = True

    def __init__(self):
        self.content = [type("Text", (), {"text": "unauthorized"})()]


class FakeClientSession:
    calls = []
    call_result = FakeCallResult()

    def __init__(self, read_stream, write_stream):
        self.read_stream = read_stream
        self.write_stream = write_stream

    async def __aenter__(self):
        return self

    async def __aexit__(self, exc_type, exc, tb):
        return False

    async def initialize(self):
        self.__class__.calls.append(("initialize",))

    async def list_tools(self):
        self.__class__.calls.append(("list_tools",))
        return FakeListResult()

    async def call_tool(self, name, arguments=None):
        self.__class__.calls.append(("call_tool", name, arguments))
        return self.__class__.call_result


class DoraemonMCPClientTest(unittest.TestCase):
    def setUp(self):
        FakeTransport.calls = []
        FakeClientSession.calls = []
        FakeClientSession.call_result = FakeCallResult()

    def test_list_tools_uses_sdk_streamable_http_client(self):
        client = DoraemonMCPClient("http://127.0.0.1:8765", "nt_test", timeout=12)

        with self.patch_sdk():
            tools = client.list_tools()

        self.assertEqual(tools, ["file.read", "host.status.get"])
        self.assertEqual(
            FakeTransport.calls,
            [
                {
                    "url": "http://127.0.0.1:8765/mcp",
                    "headers": {"Authorization": "Bearer nt_test"},
                    "timeout": 12,
                    "httpx_client_factory": client._httpx_client,
                }
            ],
        )
        self.assertEqual(FakeClientSession.calls, [("initialize",), ("list_tools",)])

    def test_call_tool_uses_sdk_session_and_returns_structured_content(self):
        client = DoraemonMCPClient("http://127.0.0.1:8765/mcp", "nt_test")

        with self.patch_sdk():
            result = client.call_tool("host.status.get", {"verbose": True})

        self.assertEqual(result, {"content": {"hostname": "demo"}})
        self.assertEqual(
            FakeClientSession.calls,
            [("initialize",), ("call_tool", "host.status.get", {"verbose": True})],
        )

    def test_tool_error_raises_mcp_error(self):
        FakeClientSession.call_result = FakeErrorResult()
        client = DoraemonMCPClient("http://127.0.0.1:8765", "bad")

        with self.patch_sdk(), self.assertRaisesRegex(MCPError, "unauthorized"):
            client.call_tool("host.status.get")

    def patch_sdk(self):
        return patch.multiple(
            doraemon_mcp_client,
            streamablehttp_client=lambda *args, **kwargs: FakeTransport(*args, **kwargs),
            ClientSession=FakeClientSession,
        )


if __name__ == "__main__":
    unittest.main()
