import json
import threading
import unittest
from http.server import BaseHTTPRequestHandler, HTTPServer

from clients.python.doraemon_mcp_client import DoraemonMCPClient, MCPError


class RecordingHandler(BaseHTTPRequestHandler):
    responses = []
    requests = []

    def do_POST(self):
        length = int(self.headers.get("Content-Length", "0"))
        body = self.rfile.read(length)
        self.__class__.requests.append(
            {
                "path": self.path,
                "authorization": self.headers.get("Authorization"),
                "content_type": self.headers.get("Content-Type"),
                "body": json.loads(body.decode("utf-8")),
            }
        )
        status, payload = self.__class__.responses.pop(0)
        encoded = json.dumps(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(encoded)))
        self.end_headers()
        self.wfile.write(encoded)

    def log_message(self, format, *args):
        return


class DoraemonMCPClientTest(unittest.TestCase):
    def setUp(self):
        RecordingHandler.responses = []
        RecordingHandler.requests = []
        self.server = HTTPServer(("127.0.0.1", 0), RecordingHandler)
        self.thread = threading.Thread(target=self.server.serve_forever)
        self.thread.daemon = True
        self.thread.start()
        self.base_url = f"http://127.0.0.1:{self.server.server_port}"

    def tearDown(self):
        self.server.shutdown()
        self.thread.join(timeout=2)
        self.server.server_close()

    def test_list_tools_sends_authenticated_json_rpc_request(self):
        RecordingHandler.responses.append(
            (
                200,
                {
                    "jsonrpc": "2.0",
                    "id": 1,
                    "result": {"tools": ["file.read", "host.status.get"]},
                },
            )
        )
        client = DoraemonMCPClient(self.base_url, "nt_test")

        tools = client.list_tools()

        self.assertEqual(tools, ["file.read", "host.status.get"])
        self.assertEqual(RecordingHandler.requests[0]["path"], "/mcp")
        self.assertEqual(RecordingHandler.requests[0]["authorization"], "Bearer nt_test")
        self.assertEqual(RecordingHandler.requests[0]["content_type"], "application/json")
        self.assertEqual(
            RecordingHandler.requests[0]["body"],
            {"jsonrpc": "2.0", "id": 1, "method": "tools/list", "params": {}},
        )

    def test_call_tool_sends_name_and_arguments(self):
        RecordingHandler.responses.append(
            (
                200,
                {
                    "jsonrpc": "2.0",
                    "id": 1,
                    "result": {"content": {"hostname": "demo"}},
                },
            )
        )
        client = DoraemonMCPClient(self.base_url + "/mcp", "nt_test")

        result = client.call_tool("host.status.get", {"verbose": True})

        self.assertEqual(result, {"content": {"hostname": "demo"}})
        self.assertEqual(
            RecordingHandler.requests[0]["body"],
            {
                "jsonrpc": "2.0",
                "id": 1,
                "method": "tools/call",
                "params": {
                    "name": "host.status.get",
                    "arguments": {"verbose": True},
                },
            },
        )

    def test_json_rpc_error_raises_mcp_error(self):
        RecordingHandler.responses.append(
            (
                200,
                {
                    "jsonrpc": "2.0",
                    "id": 1,
                    "error": {"code": -32001, "message": "unauthorized"},
                },
            )
        )
        client = DoraemonMCPClient(self.base_url, "bad")

        with self.assertRaisesRegex(MCPError, "unauthorized"):
            client.list_tools()


if __name__ == "__main__":
    unittest.main()
