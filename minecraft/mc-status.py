#!/usr/bin/env python3
"""Minecraft Server List Ping -> JSON over HTTP. Stdlib only.

Serves GET /mc/status (any path, really) with:
  {"online": true, "version": "...", "players": {"online": 0, "max": 20},
   "motd": "...", "latency_ms": 3}
or {"online": false, "error": "..."} if the server is unreachable.
"""
import json
import os
import socket
import struct
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

# MC_SERVER_* to dodge the MC_PORT=tcp://... env var k8s injects for the "mc" Service
MC_HOST = os.environ.get("MC_SERVER_HOST", "mc.minecraft.svc.cluster.local")
MC_PORT = int(os.environ.get("MC_SERVER_PORT", "25565"))


def write_varint(n: int) -> bytes:
    n &= 0xFFFFFFFF
    out = b""
    while True:
        b = n & 0x7F
        n >>= 7
        if n:
            out += bytes([b | 0x80])
        else:
            return out + bytes([b])


def read_varint(sock: socket.socket) -> int:
    n = 0
    for i in range(5):
        b = sock.recv(1)
        if not b:
            raise EOFError("connection closed mid-varint")
        n |= (b[0] & 0x7F) << (7 * i)
        if not b[0] & 0x80:
            return n
    raise ValueError("varint too long")


def motd_text(desc) -> str:
    if isinstance(desc, str):
        return desc
    if isinstance(desc, dict):
        return desc.get("text", "") + "".join(motd_text(e) for e in desc.get("extra", []))
    return ""


def ping() -> dict:
    t0 = time.monotonic()
    with socket.create_connection((MC_HOST, MC_PORT), timeout=5) as s:
        host = MC_HOST.encode()
        # handshake: id 0, protocol -1 (status), host, port, next-state 1
        hs = (write_varint(0) + write_varint(-1 & 0xFFFFFFFF)
              + write_varint(len(host)) + host
              + struct.pack(">H", MC_PORT) + write_varint(1))
        # then status request: length 1, id 0
        s.sendall(write_varint(len(hs)) + hs + b"\x01\x00")
        read_varint(s)  # total packet length
        if read_varint(s) != 0:
            raise ValueError("unexpected packet id")
        n = read_varint(s)
        data = b""
        while len(data) < n:
            chunk = s.recv(n - len(data))
            if not chunk:
                raise EOFError("connection closed mid-response")
            data += chunk
    status = json.loads(data)
    return {
        "online": True,
        "version": status.get("version", {}).get("name", "?"),
        "players": {
            "online": status.get("players", {}).get("online", 0),
            "max": status.get("players", {}).get("max", 0),
        },
        "motd": motd_text(status.get("description", "")).strip(),
        "latency_ms": round((time.monotonic() - t0) * 1000),
    }


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        try:
            body = ping()
        except Exception as e:  # noqa: BLE001 - report any failure as offline
            body = {"online": False, "error": f"{type(e).__name__}: {e}"}
        payload = json.dumps(body).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Cache-Control", "no-store")
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

    def log_message(self, *args):  # keep pod logs quiet
        pass


if __name__ == "__main__":
    ThreadingHTTPServer(("0.0.0.0", 8080), Handler).serve_forever()
