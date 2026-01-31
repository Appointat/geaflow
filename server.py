import json
import os
from typing import Any, Dict, Optional

from curl_cffi.requests import AsyncSession
from fastapi import BackgroundTasks, FastAPI, HTTPException, Request
from fastapi.responses import StreamingResponse

CONFIG_PATH = os.environ.get("ROUTER_CONFIG", "config.json")

# 逐跳 Header，代理转发时必须删除
HOP_BY_HOP = {
    "host",
    "connection",
    "keep-alive",
    "proxy-authenticate",
    "proxy-authorization",
    "te",
    "trailers",
    "transfer-encoding",
    "upgrade",
    "sec-fetch-mode",
    "sec-fetch-site",
    "sec-fetch-dest",
    # 这里的 User-Agent 等由 curl_cffi 模拟浏览器自动生成，不透传
    "user-agent",
    "sec-ch-ua",
    "sec-ch-ua-mobile",
    "sec-ch-ua-platform",
    "accept-encoding",
    "content-length",
}

DEBUG = os.environ.get("ROUTER_DEBUG", "0") in {"1", "true", "TRUE", "yes", "YES"}

app = FastAPI(title="Simple OpenAI-Compatible Router (Full Proxy)")


def load_config() -> Dict[str, Any]:
    if not os.path.exists(CONFIG_PATH):
        print(f"[Warning] Config file {CONFIG_PATH} not found.")
        return {}
    with open(CONFIG_PATH, "r", encoding="utf-8") as f:
        return json.load(f)


def check_local_apikey(req: Request, cfg: Dict[str, Any]) -> None:
    expected = (cfg.get("APIKEY") or "").strip()
    if not expected:
        return
    auth = (req.headers.get("authorization") or "").strip()
    if auth != f"Bearer {expected}":
        raise HTTPException(status_code=401, detail="Unauthorized")


def find_provider_by_model(cfg: Dict[str, Any], model: Optional[str]) -> Dict[str, Any]:
    if not model:
        raise HTTPException(
            status_code=400,
            detail="Missing model (use JSON body.model or ?model=... or X-Model)",
        )
    for p in cfg.get("Providers", []) or []:
        if model in (p.get("models") or []):
            if not p.get("api_base_url"):
                raise HTTPException(
                    status_code=500,
                    detail=f"Provider {p.get('name')} missing api_base_url",
                )
            return p
    raise HTTPException(status_code=404, detail=f"Model not found in config: {model}")


def set_header_case_insensitive(headers: Dict[str, str], key: str, value: str) -> None:
    key_lower = key.lower()
    for k in list(headers.keys()):
        if k.lower() == key_lower:
            headers.pop(k)
    headers[key] = value


def remove_header_case_insensitive(headers: Dict[str, str], key: str) -> None:
    key_lower = key.lower()
    for k in list(headers.keys()):
        if k.lower() == key_lower:
            headers.pop(k)


def upstream_headers(
    req: Request, provider: Dict[str, Any], cfg: Dict[str, Any]
) -> Dict[str, str]:
    headers = dict(req.headers)

    # 1. 移除 HOP_BY_HOP 和可能导致指纹冲突的 Header
    for h in HOP_BY_HOP:
        remove_header_case_insensitive(headers, h)

    # 2. 根据 Config 移除 Header
    strip_headers_list = provider.get("strip_headers")
    if strip_headers_list is None:
        strip_headers_list = cfg.get("STRIP_HEADERS") or []

    for header_to_strip in strip_headers_list:
        # 注意：Content-Length 我们在 proxy_all 里会重新加回去，这里删了也没事
        remove_header_case_insensitive(headers, header_to_strip)

    # 3. 设置 Authorization
    key = (provider.get("api_key") or "").strip()
    if key and key.lower() not in {"不需要apikey", "none", "null"}:
        set_header_case_insensitive(headers, "Authorization", f"Bearer {key}")
    else:
        remove_header_case_insensitive(headers, "authorization")

    # 4. 强制 Content-Type (防止有些客户端没发)
    set_header_case_insensitive(headers, "Content-Type", "application/json")

    return headers


def make_client(
    cfg: Dict[str, Any], provider: Optional[Dict[str, Any]] = None
) -> AsyncSession:
    timeout_ms = int(cfg.get("API_TIMEOUT_MS", 600000))  # 默认 10 分钟超时

    proxy = None
    if provider is not None:
        proxy = (provider.get("proxy_url") or "").strip() or None
    if not proxy:
        proxy = (cfg.get("PROXY_URL") or "").strip() or None

    if DEBUG and proxy:
        print(f"[router] Using proxy: {proxy}")

    return AsyncSession(
        timeout=timeout_ms / 1000,
        proxy=proxy,
        allow_redirects=True,
        impersonate="chrome120",  # 伪装成 Chrome 120 绕过 Cloudflare
        verify=False,
    )


@app.get("/health")
async def health():
    return {"ok": True}


@app.get("/v1/models")
async def models(req: Request):
    cfg = load_config()
    check_local_apikey(req, cfg)
    data = []
    for p in cfg.get("Providers", []) or []:
        for m in p.get("models") or []:
            data.append(
                {"id": m, "object": "model", "owned_by": p.get("name", "provider")}
            )
    return {"object": "list", "data": data}


@app.api_route(
    "/{path:path}", methods=["GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"]
)
async def proxy_all(path: str, req: Request, background_tasks: BackgroundTasks):
    cfg = load_config()
    check_local_apikey(req, cfg)

    # 1. 提取 Model 和 Body
    model = req.query_params.get("model") or req.headers.get("x-model")
    content_type = (req.headers.get("content-type") or "").lower()
    body_bytes = await req.body()

    if "application/json" in content_type and body_bytes:
        try:
            obj = json.loads(body_bytes.decode("utf-8"))
            if not model:
                model = obj.get("model")

            if model == "gpt-5.2-long":
                obj["reasoning_effort"] = "high"
                obj["verbosity"] = "high"
                obj["model"] = "gpt-5.2"
                body_bytes = json.dumps(obj).encode("utf-8")

            elif model == "gpt-5.2-high":
                obj["reasoning_effort"] = "high"
                obj["verbosity"] = "low"
                obj["model"] = "gpt-5.2"
                body_bytes = json.dumps(obj).encode("utf-8")

        except json.JSONDecodeError:
            pass

    provider = find_provider_by_model(cfg, model)
    upstream_url = provider["api_base_url"]

    # 2. 准备请求头
    headers = upstream_headers(req, provider, cfg)

    if DEBUG:
        print(f"[router] forwarding to: {upstream_url}")
        print(f"[router] request to: {provider.get('name')} {model}")
        print(f"[router] headers: {headers}")

    # 3. 创建 Session (注意：不使用 async with，由 background_tasks 关闭)
    session = make_client(cfg, provider)

    # 确保流式传输完成后，连接才会被关闭
    background_tasks.add_task(session.close)

    try:
        response = await session.request(
            method=req.method,
            url=upstream_url,
            headers=headers,
            params=dict(req.query_params),
            data=body_bytes,  # 使用 data 透传原始 bytes
            stream=True,
        )

        if DEBUG:
            print(f"[router] upstream status: {response.status_code}")

        # 4. 准备响应头 (Upstream -> Client)
        resp_headers = {}

        # curl_cffi 已自动解压数据，如果透传 gzip header，客户端(Alma)会二次解压导致报错
        HEADERS_TO_DROP = {
            "content-encoding",
            "transfer-encoding",
            "content-length",
            "connection",
        }

        for k, v in response.headers.items():
            if k.lower() not in HOP_BY_HOP and k.lower() not in HEADERS_TO_DROP:
                resp_headers[k] = v

        # 5. 错误处理
        if response.status_code >= 400:
            content = await response.acontent()
            if DEBUG:
                print(f"[router] error body: {content[:500]}")
            return StreamingResponse(
                iter([content]), status_code=response.status_code, headers=resp_headers
            )

        # 6. 流式透传
        async def iter_bytes():
            try:
                async for chunk in response.aiter_content():
                    yield chunk
            except Exception as e:
                if DEBUG:
                    print(f"[router] stream error: {e}")
            # session.close() 会在 background_tasks 中执行

        return StreamingResponse(
            iter_bytes(),
            status_code=response.status_code,
            headers=resp_headers,
            media_type=response.headers.get("content-type"),
            background=background_tasks,  # 绑定生命周期
        )

    except Exception as e:
        # 如果请求还没发出去就崩了，手动关闭 session
        await session.close()
        print(f"[router] internal error: {e}")
        raise HTTPException(status_code=502, detail=str(e))
