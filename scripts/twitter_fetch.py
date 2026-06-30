#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
PERIPHERA 实时新闻旁路抓取：用 twscrape 从 X(Twitter) 拉指定账号的最新推文，
输出统一 JSON 到 stdout，供 Go 后台 cron 解析灌入新闻缓存。

设计：
  - cookie 自读项目根 .env（x_auth_token / x_ct0），不经过 Go 进程，最小暴露面。
  - twscrape 账号库落在 scripts/.twscrape_accounts.db（已 gitignore）。
  - 幂等：每次运行确保账号存在并激活；失败降级（输出 ok=false，Go 保留上次缓存）。

运行（venv）：
  scripts/venv/bin/python scripts/twitter_fetch.py [--limit N] [--targets elonmusk,aleabitoreddit]

输出（stdout，UTF-8）：
  {"ok": true, "items": [ {id, source, source_name, author, title, summary,
                           url, published_at(ms), media:[...], tags:[...]} ], "errors":[...]}
  失败：{"ok": false, "error": "..."}（退出码非 0）
"""
import argparse
import asyncio
import json
import os
import sys

# ---- 路径与常量 ----
SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
PROJECT_ROOT = os.path.dirname(SCRIPT_DIR)
ENV_FILE = os.path.join(PROJECT_ROOT, ".env")
ACCOUNTS_DB = os.path.join(SCRIPT_DIR, ".twscrape_accounts.db")
POOL_USERNAME = "periphera_pool1"  # twscrape 账号库内的 key（与目标账号无关）

# 目标账号 → 新闻来源映射（与 PERIPHERA_API.md / seed 来源 code 对齐）
TARGET_SOURCES = {
    "elonmusk": {"source": "musk", "source_name": "马斯克", "author": "Elon Musk"},
    "aleabitoreddit": {"source": "baimao", "source_name": "白毛女神", "author": "白毛女神"},
}


def load_env_cookies():
    """从项目根 .env 解析 x_auth_token / x_ct0，拼成 twscrape 需要的 cookie 串。"""
    if not os.path.isfile(ENV_FILE):
        raise RuntimeError(f".env not found: {ENV_FILE}")
    vals = {}
    with open(ENV_FILE, "r", encoding="utf-8") as fh:
        for line in fh:
            line = line.strip()
            if not line or line.startswith("#") or "=" not in line:
                continue
            if line.startswith("export "):
                line = line[len("export "):]
            k, _, v = line.partition("=")
            vals[k.strip()] = v.strip().strip('"').strip("'")
    auth = vals.get("x_auth_token") or os.getenv("x_auth_token")
    ct0 = vals.get("x_ct0") or os.getenv("x_ct0")
    if not auth or not ct0:
        raise RuntimeError("x_auth_token / x_ct0 missing in .env")
    # 国内环境 x.com 需走代理：.env 里配 X_PROXY=http://127.0.0.1:7890（或 socks5://...）
    proxy = vals.get("X_PROXY") or os.getenv("X_PROXY") or os.getenv("HTTPS_PROXY") or os.getenv("https_proxy")
    return f"auth_token={auth}; ct0={ct0}", (proxy or None)


def to_ms(dt):
    try:
        return int(dt.timestamp() * 1000)
    except Exception:
        return 0


def collect_media(tw):
    urls = []
    try:
        for p in (tw.media.photos or []):
            if getattr(p, "url", None):
                urls.append(p.url)
        for v in (tw.media.videos or []):
            if getattr(v, "thumbnailUrl", None):
                urls.append(v.thumbnailUrl)
        for a in (tw.media.animated or []):
            if getattr(a, "thumbnailUrl", None):
                urls.append(a.thumbnailUrl)
    except Exception:
        pass
    return urls


def make_title(text):
    text = (text or "").strip()
    if not text:
        return "(无文本)"
    first = text.splitlines()[0].strip()
    return first[:80] + ("…" if len(first) > 80 else "")


async def fetch(limit, targets):
    from twscrape import API, gather

    cookies, proxy = load_env_cookies()

    # twscrape 0.19.x 的 XClIdGen 内部 client 不透传 proxy（xclid._make_client 写死无代理），
    # 国内直连 x.com 必失败。这里 monkeypatch 注入代理，主请求链路本身已支持代理。
    if proxy:
        import twscrape.xclid as _xclid
        _mk = _xclid._make_http_client
        _xclid._make_client = lambda: _mk(headers={"user-agent": "@chrome"}, proxy=proxy)

    api = API(ACCOUNTS_DB, proxy=proxy)

    # 幂等添加账号（已存在则忽略错误），再用 cookie 激活
    try:
        await api.pool.add_account(
            username=POOL_USERNAME, password="x", email="x@x.x",
            email_password="x", cookies=cookies, proxy=proxy,
        )
    except Exception:
        pass  # 已存在（cookie 轮换时删除 scripts/.twscrape_accounts.db 重建）
    await api.pool.login_all()

    items, errors = [], []
    for login in targets:
        meta = TARGET_SOURCES.get(login, {"source": login, "source_name": login, "author": login})
        try:
            user = await api.user_by_login(login)
            if not user:
                errors.append(f"{login}: user not found")
                continue
            tweets = await gather(api.user_tweets(user.id, limit=limit))
            for tw in tweets:
                text = tw.rawContent or ""
                items.append({
                    "id": "x_" + str(tw.id),
                    "source": meta["source"],
                    "source_name": meta["source_name"],
                    "author": meta["author"],
                    "title": make_title(text),
                    "summary": text.strip(),
                    "url": tw.url,
                    "published_at": to_ms(tw.date),
                    "media": collect_media(tw),
                    "tags": [h for h in (tw.hashtags or [])][:5],
                })
        except Exception as e:
            errors.append(f"{login}: {e}")

    return {"ok": True, "items": items, "errors": errors}


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--limit", type=int, default=int(os.getenv("X_FETCH_LIMIT", "20")))
    ap.add_argument("--targets", default=os.getenv("X_TARGETS", "elonmusk,aleabitoreddit"))
    args = ap.parse_args()
    targets = [t.strip() for t in args.targets.split(",") if t.strip()]

    try:
        result = asyncio.run(fetch(args.limit, targets))
    except Exception as e:
        print(json.dumps({"ok": False, "error": str(e)}, ensure_ascii=False))
        sys.exit(1)

    print(json.dumps(result, ensure_ascii=False))
    # 全部目标都失败才算失败
    if not result["items"] and result["errors"]:
        sys.exit(2)


if __name__ == "__main__":
    main()
