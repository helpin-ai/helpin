#!/usr/bin/env python3
"""Serve the local SDK and proxy browser events to the capture process."""

from __future__ import annotations

import mimetypes
import os
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen


SDK_DIST = Path(os.environ["HELPIN_SDK_DIST"]).resolve()
CAPTURE_URL = os.environ.get("HELPIN_CAPTURE_URL", "http://127.0.0.1:3000").rstrip("/")
LISTEN_HOST = os.environ.get("HELPIN_EVENT_PROXY_HOST", "0.0.0.0")
LISTEN_PORT = int(os.environ.get("HELPIN_EVENT_PROXY_PORT", "8095"))
HOP_BY_HOP_HEADERS = {
    "connection",
    "content-length",
    "host",
    "keep-alive",
    "proxy-authenticate",
    "proxy-authorization",
    "te",
    "trailers",
    "transfer-encoding",
    "upgrade",
}


class EventProxyHandler(BaseHTTPRequestHandler):
    server_version = "HelpinLocalEventProxy/1.0"

    def do_GET(self) -> None:  # noqa: N802
        if self.path.startswith("/sdk/"):
            self._serve_sdk()
            return
        if self.path.startswith("/health/"):
            self._proxy()
            return
        self.send_error(404)

    def do_POST(self) -> None:  # noqa: N802
        if self.path.startswith("/api/v1/event") or self.path.startswith("/api."):
            self._proxy()
            return
        self.send_error(404)

    def do_OPTIONS(self) -> None:  # noqa: N802
        if self.path.startswith("/api/v1/event") or self.path.startswith("/api."):
            self._proxy()
            return
        self.send_error(404)

    def _serve_sdk(self) -> None:
        relative = self.path.split("?", 1)[0].removeprefix("/sdk/")
        candidate = (SDK_DIST / relative).resolve()
        if SDK_DIST not in candidate.parents or not candidate.is_file():
            self.send_error(404)
            return

        body = candidate.read_bytes()
        content_type = mimetypes.guess_type(candidate.name)[0] or "application/octet-stream"
        self.send_response(200)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(body)))
        self.send_header("Access-Control-Allow-Origin", "*")
        cache_age = "300" if candidate.name == "lib.js" else "31536000, immutable"
        self.send_header("Cache-Control", f"public, max-age={cache_age}")
        self.end_headers()
        self.wfile.write(body)

    def _proxy(self) -> None:
        content_length = int(self.headers.get("Content-Length", "0"))
        body = self.rfile.read(content_length) if content_length else None
        headers = {
            name: value
            for name, value in self.headers.items()
            if name.lower() not in HOP_BY_HOP_HEADERS
        }
        request = Request(
            f"{CAPTURE_URL}{self.path}",
            data=body,
            headers=headers,
            method=self.command,
        )

        try:
            response = urlopen(request, timeout=30)
        except HTTPError as error:
            response = error
        except URLError as error:
            self.send_error(502, f"capture unavailable: {error.reason}")
            return

        response_body = response.read()
        self.send_response(response.status)
        for name, value in response.headers.items():
            if name.lower() not in HOP_BY_HOP_HEADERS:
                self.send_header(name, value)
        self.send_header("Content-Length", str(len(response_body)))
        self.end_headers()
        self.wfile.write(response_body)

    def log_message(self, message: str, *args: object) -> None:
        print(f"{self.address_string()} - {message % args}", flush=True)


if __name__ == "__main__":
    print(f"Local event proxy listening on http://{LISTEN_HOST}:{LISTEN_PORT}", flush=True)
    ThreadingHTTPServer((LISTEN_HOST, LISTEN_PORT), EventProxyHandler).serve_forever()
