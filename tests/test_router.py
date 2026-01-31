import json
from pathlib import Path

import pytest
from httpx import ASGITransport, AsyncClient


@pytest.fixture()
def tmp_config(tmp_path: Path) -> Path:
    cfg = {
        "HOST": "127.0.0.1",
        "PORT": 5840,
        "APIKEY": "local-key",
        "API_TIMEOUT_MS": 600000,
        "PROXY_URL": "",
        "Providers": [
            {
                "name": "p1",
                "api_base_url": "http://upstream.test/any",
                "api_key": "upstream-key",
                "models": ["m1"],
            }
        ],
    }
    p = tmp_path / "config.json"
    p.write_text(json.dumps(cfg), encoding="utf-8")
    return p


@pytest.fixture()
def app(monkeypatch, tmp_config: Path):
    # server reads CONFIG_PATH at import time -> reload after setting env.
    monkeypatch.setenv("ROUTER_CONFIG", str(tmp_config))

    import importlib

    import server

    server = importlib.reload(server)
    return server.app


@pytest.fixture()
async def client(app):
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as ac:
        yield ac


class FakeUpstreamResponse:
    def __init__(self, status_code: int, headers: dict, chunks: list[bytes]):
        self.status_code = status_code
        self.headers = headers
        self._chunks = chunks

    async def aiter_content(self):
        for c in self._chunks:
            yield c

    async def acontent(self) -> bytes:
        return b"".join(self._chunks)

    async def aclose(self):
        return


class FakeUpstreamClient:
    """Returned by server.make_client() in tests.

    This fakes the subset of curl_cffi.requests.AsyncSession used by server.py.
    """

    def __init__(self, handler):
        self._handler = handler

    async def close(self):
        return

    async def request(
        self,
        method: str,
        url: str,
        headers=None,
        params=None,
        data=None,
        stream: bool = False,
        **_kwargs,
    ):
        req = {
            "method": method,
            "url": str(url),
            "headers": dict(headers or {}),
            "params": dict(params or {}),
            "content": data or b"",
        }
        return await self._handler(req, stream)


@pytest.mark.asyncio
async def test_models_requires_auth(client: AsyncClient):
    r = await client.get("/v1/models")
    assert r.status_code == 401


@pytest.mark.asyncio
async def test_models_list(client: AsyncClient):
    r = await client.get("/v1/models", headers={"Authorization": "Bearer local-key"})
    assert r.status_code == 200
    data = r.json()
    assert data["object"] == "list"
    assert any(m["id"] == "m1" for m in data["data"])


@pytest.mark.asyncio
async def test_json_routing_and_header_rewrite(monkeypatch, client: AsyncClient):
    import server as srv

    async def handler(req: dict, stream: bool):
        assert stream is True
        assert req["url"].startswith("http://upstream.test/any")
        assert req["headers"].get("Authorization") == "Bearer upstream-key"
        # client-specific headers should NOT be forwarded
        assert "x-stainless-retry-count" not in {k.lower() for k in req["headers"]}


        body = json.loads(req["content"].decode("utf-8"))
        assert body["model"] == "m1"

        return FakeUpstreamResponse(
            200,
            {"content-type": "application/json"},
            [b"{\"ok\":true}"]
        )

    monkeypatch.setattr(srv, "make_client", lambda cfg, provider=None: FakeUpstreamClient(handler))

    r = await client.post(
        "/v1/chat/completions",
        headers={"Authorization": "Bearer local-key", "Content-Type": "application/json"},
        json={"model": "m1", "messages": [{"role": "user", "content": "hi"}]},
    )

    assert r.status_code == 200
    assert r.json() == {"ok": True}


@pytest.mark.asyncio
async def test_binary_body_routing_with_query_model(monkeypatch, client: AsyncClient):
    import server as srv

    async def handler(req: dict, stream: bool):
        assert stream is True
        assert req["url"].startswith("http://upstream.test/any")
        assert req["headers"].get("Authorization") == "Bearer upstream-key"
        assert req["content"] == b"%PDF-1.7 fake"
        return FakeUpstreamResponse(200, {"content-type": "application/pdf"}, [b"OK"])

    monkeypatch.setattr(srv, "make_client", lambda cfg, provider=None: FakeUpstreamClient(handler))

    r = await client.post(
        "/v1/files?model=m1",
        headers={"Authorization": "Bearer local-key", "Content-Type": "application/pdf"},
        content=b"%PDF-1.7 fake",
    )

    assert r.status_code == 200
    assert r.content == b"OK"
    assert r.headers["content-type"].startswith("application/pdf")


@pytest.mark.asyncio
async def test_streaming_passthrough(monkeypatch, client: AsyncClient):
    import server as srv

    async def handler(req: dict, stream: bool):
        assert stream is True
        return FakeUpstreamResponse(
            200,
            {"content-type": "text/event-stream"},
            [b"data: a\n\n", b"data: b\n\n"],
        )

    monkeypatch.setattr(srv, "make_client", lambda cfg, provider=None: FakeUpstreamClient(handler))

    r = await client.post(
        "/v1/chat/completions",
        headers={"Authorization": "Bearer local-key", "Content-Type": "application/json"},
        json={"model": "m1", "stream": True, "messages": [{"role": "user", "content": "hi"}]},
    )

    assert r.status_code == 200
    assert r.headers["content-type"].startswith("text/event-stream")
    assert r.text == "data: a\n\ndata: b\n\n"


@pytest.mark.asyncio
async def test_header_stripping_provider_override(monkeypatch, tmp_path: Path):
    """Verify provider-level `strip_headers` overrides global and default lists."""
    import json

    import server as srv

    # Config with global STRIP_HEADERS and two providers:
    # p_override: has its own `strip_headers` (should ignore global/default)
    # p_default: inherits global `STRIP_HEADERS`
    # p_nostrip: has empty `strip_headers` to disable stripping
    cfg = {
        "APIKEY": "local-key",
        "STRIP_HEADERS": ["x-global-strip"],
        "Providers": [
            {
                "name": "p_override",
                "api_base_url": "http://upstream.test/override",
                "api_key": "upstream-key",
                "models": ["m_override"],
                "strip_headers": ["x-provider-strip"], # This should take precedence
            },
            {
                "name": "p_default",
                "api_base_url": "http://upstream.test/default",
                "api_key": "upstream-key",
                "models": ["m_default"],
                # No strip_headers, so should use global + default
            },
            {
                "name": "p_nostrip",
                "api_base_url": "http://upstream.test/nostrip",
                "api_key": "upstream-key",
                "models": ["m_nostrip"],
                "strip_headers": [], # Empty list should disable stripping
            },
        ],
    }
    p = tmp_path / "config_override.json"
    p.write_text(json.dumps(cfg), encoding="utf-8")
    monkeypatch.setenv("ROUTER_CONFIG", str(p))

    import importlib
    importlib.reload(srv)

    captured_headers = {}

    async def handler(req: dict, stream: bool):
        nonlocal captured_headers
        captured_headers = req["headers"]
        return FakeUpstreamResponse(200, {}, [b"{}"])

    monkeypatch.setattr(srv, "make_client", lambda cfg, provider=None: FakeUpstreamClient(handler))

    transport = ASGITransport(app=srv.app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:

        # --- Test Case 1: Provider with specific `strip_headers` ---
        await client.post(
            "/v1/chat/completions",
            headers={
                "Authorization": "Bearer local-key",
                "User-Agent": "test-ua",
                "Origin": "http://evil.com",
                "X-Global-Strip": "value",
                "X-Provider-Strip": "value",
                "X-Safe-Header": "value",
            },
            json={"model": "m_override", "messages": []},
        )

        h_keys = {k.lower() for k in captured_headers}
        assert "x-provider-strip" not in h_keys # Provider list is used
        assert "x-global-strip" in h_keys     # Global list is ignored
        assert "origin" in h_keys             # Default list is ignored
        assert "x-safe-header" in h_keys
        assert "user-agent" not in h_keys  # server.py always strips UA (HOP_BY_HOP)


        # --- Test Case 2: Provider inheriting default `strip_headers` ---
        await client.post(
            "/v1/chat/completions",
            headers={
                "Authorization": "Bearer local-key",
                "User-Agent": "test-ua",
                "Origin": "http://evil.com", # Should be stripped by default
                "X-Global-Strip": "value",   # Should be stripped by global config
                "X-Safe-Header": "value",
            },
            json={"model": "m_default", "messages": []},
        )

        h_keys = {k.lower() for k in captured_headers}
        assert "origin" in h_keys  # origin is NOT stripped because global config overrides default
        assert "x-global-strip" not in h_keys  # Global config is used when provider has no strip_headers
        assert "user-agent" not in h_keys  # server.py always strips UA (HOP_BY_HOP)
        assert "x-safe-header" in h_keys


        # --- Test Case 3: Provider with empty `strip_headers` (no stripping) ---
        await client.post(
            "/v1/chat/completions",
            headers={
                "Authorization": "Bearer local-key",
                "User-Agent": "test-ua",
                "Origin": "http://evil.com",
                "X-Global-Strip": "value",
                "X-Safe-Header": "value",
            },
            json={"model": "m_nostrip", "messages": []},
        )

        h_keys = {k.lower() for k in captured_headers}
        # Nothing is stripped (except special cases like Auth/UA)
        assert "origin" in h_keys
        assert "x-global-strip" in h_keys
        assert "x-safe-header" in h_keys
        assert "user-agent" not in h_keys  # server.py always strips UA (HOP_BY_HOP)
