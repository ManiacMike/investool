# PERIPHERA 后端接口契约 (v1)

> 投资资讯指挥中心（/portal、/news、/research）的数据接入契约。
> 前端（Codex）按本文件对接；后端（Claude）按本文件实现。**字段尽量与当前前端 mock 同名，降低改造成本。**

- 传输方式：**轮询 REST**（前端定时 GET）。无需 SSE/WebSocket。
- 行情源：扩展 `datacenter/sina`、`datacenter/qq` 拉实时行情。
- 新闻/宏观源：后端逐步接入真实源（详见 TODO）。
- 存储：**按板块分别定** —— 行情/新闻走内存缓存+cron 刷新；研报做轻量持久化（JSON 文件 / MySQL）。

---

## 0. 通用约定

### 0.1 Base URL & 版本
```
/api/v1
```

### 0.2 统一响应信封（复用 routes/response）
所有接口返回：
```json
{ "code": 0, "msg": "Success", "data": <payload> }
```
- `code == 0` 成功（`routes/response.CodeSuccess`）；非 0 为错误（见 0.5）。
- 业务数据一律在 `data` 中。

### 0.3 时间戳
- 所有机器时间戳字段统一用 **Unix 毫秒（UTC）**，字段后缀 `_at`/`_ts`（如 `published_at`、`server_ts`）。
- 人类可读的展示时间（如研报 `publish_time = "2026-06-27"`）保留字符串，由后端给定。
- 前端的「3 分钟前」等相对时间由前端根据 `*_at` 自行计算。

### 0.4 增量拉取（红点/未推送）
列表型接口（news、research）支持游标：
- 请求 `?since=<unix_ms>`：仅返回 `published_at > since` 的新条目（用于红点未读、"↑ N 条新推送"）。
- 不传 `since`：返回最新 `limit` 条（首屏）。
- 响应固定带 `server_ts`（本次服务器时间）与 `latest_ts`（本批最新条目时间）。前端下次轮询用上次的 `latest_ts` 作为 `since`。

### 0.5 错误码
| code | 含义 | 说明 |
|------|------|------|
| 0 | Success | 成功 |
| -1 | Failure | 通用失败 |
| 1 | Invalid Param | 参数错误（如非法 code） |
| 2 | Not Found | 资源不存在 |
| 3 | Unknown Error | 内部错误 |

> 实际取值来自 `routes/response.errcode`（`failure=-1, success=0, invalidParam=1, notFound=2, unknownError=3`）。

> 错误时 HTTP 状态仍为 200，靠 `code` 区分（沿用项目现有 `response` 约定）。`data` 为 `null`。

### 0.6 缓存与限频
- 行情/宏观接口由后端内存缓存，前端轮询直接命中缓存，不击穿到上游。
- 已有 `ratelimiter` 中间件（config `ratelimiter.enable=true`），新接口同样受限频保护。

### 0.7 推荐轮询频率（前端）
| 模块 | 接口 | 建议间隔 |
|------|------|----------|
| 顶部行情条 ticker | `/ticker` | 3–5s |
| 金银铜油 / 暗盘 | `/commodities` | 3–5s |
| 原油分时 | `/commodities/crude` | 3–5s |
| 全球股指 | `/markets/indices` | 3–5s |
| 加密 | `/crypto` | 3–5s |
| 实时新闻 | `/news?since=` | 8–10s |
| 外资研报 | `/research?since=` | 30–60s |
| 美股板块涨幅 | `/markets/us-sectors` | 60s |
| 霍尔木兹 | `/macro/hormuz` | 5–15min |
| FedWatch | `/macro/fedwatch` | 5–15min |
| AI 简报 | `/ai/briefing` | 进入时 + 每小时 |

---

## 1. 实时新闻

> **数据源现状**：`musk`/`baimao` 已接真实源（twscrape 抓 X，cron 300s 刷新，id 形如 `x_<tweetid>`）；`reuters`/`bloomberg` 仍为占位（`/news/sources` 中 count=0），待接 RSS/聚合源。结构不变。

### `GET /api/v1/news`
Query：
| 参数 | 类型 | 默认 | 说明 |
|------|------|------|------|
| `source` | string | `all` | 来源筛选：`all` / `reuters` / `bloomberg` / `musk` / `baimao` |
| `since` | int(ms) | - | 增量游标，仅返回更新的 |
| `limit` | int | 30 | 最大 100 |

响应 `data`：
```json
{
  "items": [
    {
      "id": "n_8f3a1c",
      "source": "reuters",
      "source_name": "路透社",
      "color": "#D0643E",
      "tag": "RT",
      "title": "霍尔木兹海峡油轮通行量环比下降，布伦特原油盘中跳涨逾 2%",
      "summary": "地缘紧张推升供给担忧，交易员关注护航与保险费率变化。",
      "url": "https://...",
      "published_at": 1751049600000,
      "hot": true,
      "tags": ["原油", "霍尔木兹"]
    }
  ],
  "server_ts": 1751049605000,
  "latest_ts": 1751049600000
}
```
> 前端映射：`body ← summary`，相对时间 ← `published_at`。

### `GET /api/v1/news/sources`
左侧来源栏。响应 `data`：
```json
{
  "sources": [
    { "code": "reuters", "name": "路透社", "tag": "RT", "color": "#D0643E", "desc": "全球财经快讯", "count": 42 }
  ]
}
```

---

## 2. 外资研报

> 字段名与项目既有研报结构（`zen-research-report` 的 `REPORT_FIELDS`）一致。
> **存储：MySQL `periphera.research_reports`（已建库建表）。** 去重键 `dedup_key`＝`video_id`（非空时）或 `research_target|publish_time|institution_name` 规范化。

### `GET /api/v1/research`
Query：`rating`（`all`/买入/增持/中性/减持）、`institution`、`q`（关键词）、`since`、`limit`（默认 30）。

响应 `data`：
```json
{
  "items": [
    {
      "id": "r_1a2b3c",
      "publish_time": "2026-06-27",
      "industry_category": "能源",
      "institution_name": "高盛 GS",
      "research_target": "能源板块",
      "report_type": "行业深度",
      "core_content": "地缘风险溢价回归，上调原油全年中枢至 85 美元……",
      "target_price": "—",
      "investment_rating": "增持",
      "rating_change": "上调",
      "core_catalyst": "霍尔木兹局势 + OPEC+ 减产纪律",
      "core_risk_warning": "需求走弱、地缘缓和导致溢价回吐",
      "earnings_forecast_adjustment": "上修 10%",
      "video_id": "7652965352200031538",
      "author": "白毛女神",
      "source_url": "https://www.douyin.com/video/7652965352200031538",
      "created_at": 1751000000000
    }
  ],
  "summary": { "total": 9, "upgrades": 3, "buy": 5, "targets": 8 },
  "server_ts": 1751049605000,
  "latest_ts": 1751000000000
}
```

### `GET /api/v1/research/:id`
单条，`data` 为上面的单个 item。

### （P1，录入页对接）研报写入
迁移 `/invest/zen-research-report` 录入页从 localStorage 到后端：
- `POST /api/v1/research` body=研报对象（无 `id`/`created_at`，后端生成）→ `data` 返回创建后的 item。去重：有 `video_id` 按 `video_id`，否则按 `research_target + publish_time + institution_name`。
- `PUT /api/v1/research/:id` 更新。
- `DELETE /api/v1/research/:id` 删除。
- `POST /api/v1/research/import` body=`{ "items": [...] }` 批量导入（兼容现有粘贴 JSON 能力）→ `data` 返回 `{ added, updated }`。

> **抖音→豆包脚本接入**：`scripts/douyin_to_report.mjs` 抖音链接 → 豆包专家模型抽取结构化研报 → **POST 到 `/api/v1/research/import`**（落 MySQL，按 `video_id` 去重 upsert）。运行：`node scripts/douyin_to_report.mjs "<抖音链接>"`，可用环境变量 `API_BASE`（默认 `http://localhost:4869`）覆盖后台地址。
> DB 凭据走环境变量 `PERIPHERA_MYSQL_PASSWORD`（或完整 `PERIPHERA_MYSQL_DSN`），不入 `config.toml`。

---

## 3. 全球股市

### `GET /api/v1/markets/indices`
Query：`codes`（逗号分隔，默认 `SPX,IXIC,N225,KOSPI,TWSE`）。

响应 `data`：
```json
{
  "items": [
    {
      "code": "SPX", "name": "标普500", "flag": "🇺🇸",
      "value": 5487.2, "change": 33.1, "change_pct": 0.61,
      "prev_close": 5454.1, "is_open": true,
      "spark": [5450.1, 5455.3, 5460.0],
      "updated_at": 1751049600000
    }
  ],
  "server_ts": 1751049605000
}
```
> 前端映射：`chg ← change_pct`，迷你图 ← `spark`。
> 代码表：`SPX`=标普500、`IXIC`=纳指、`DJI`=道指、`N225`=日经225、`KOSPI`=韩国综合、`TWSE`=台湾加权。台/韩覆盖需后端验证，缺失时返回最近可得值并标 `is_open:false`。

### `GET /api/v1/markets/us-sectors`
美股隔夜板块涨幅。响应 `data`：
```json
{
  "items": [
    { "name": "能源", "change_pct": 2.4 },
    { "name": "信息技术", "change_pct": 1.8 }
  ],
  "trade_date": "2026-06-27",
  "updated_at": 1751049600000
}
```
> 前端映射：`v ← change_pct`。排序由前端做（也可后端预排）。

---

## 4. 外围数据（商品）

### `GET /api/v1/commodities`
金银铜油 + 暗金/暗油（夜盘/电子盘）。Query：`codes`（默认 `XAU,XAG,HG,WTI,XAU_AH,WTI_AH`）。

响应 `data`：
```json
{
  "items": [
    {
      "code": "XAU", "name": "黄金", "price": 2356.4,
      "change": 19.2, "change_pct": 0.82,
      "dark": false, "spark": [2350.1, 2352.0, 2356.4],
      "unit": "USD/oz", "updated_at": 1751049600000
    },
    {
      "code": "XAU_AH", "name": "暗金", "price": 2361.0,
      "change": 4.6, "change_pct": 0.20, "dark": true,
      "spark": [], "updated_at": 1751049600000
    }
  ],
  "server_ts": 1751049605000
}
```
> 代码表：`XAU`金 (`hf_GC`, USD/oz)、`XAG`银 (`hf_SI`, USD/oz)、`HG`铜 (`hf_HG`, **美分/磅** ≈ 628)、`WTI`原油 (`hf_CL`, USD/bbl)。`unit` 字段标明单位，前端按 `unit` 展示即可。
> `*_AH`=暗盘/夜盘；若 sina/qq 无夜盘字段，后端先返回 `dark:true` + 空 `spark` + 主盘价占位，TODO 标注，后补真实夜盘源。

### `GET /api/v1/commodities/crude`
原油实时分时（WTI + Brent）。响应 `data`：
```json
{
  "wti":   { "price": 81.92, "change_pct": 2.31 },
  "brent": { "price": 86.10, "change_pct": 1.98 },
  "labels": ["", "", ""],
  "wti_series":   [81.0, 81.3, 81.92],
  "brent_series": [85.2, 85.6, 86.10],
  "updated_at": 1751049600000
}
```
> sina：WTI `hf_CL`、Brent `hf_OIL`。分时序列：后端维护近 N 点滑动窗口（内存）。

---

## 5. 加密行情

### `GET /api/v1/crypto`
Query：`codes`（默认 `BTC,ETH,PAXG,XAUT`）。响应 `data`：
> `PAXG`/`XAUT` 为黄金锚定币（Binance `PAXGUSDT`/`XAUTUSDT`，CoinGecko 兜底），可作「暗金」近似；`BTC` 走 sina，`ETH` 暂回退 seed。
```json
{
  "items": [
    { "code": "BTC", "name": "BTC", "price": 61240.0, "change_pct": 1.9, "spark": [60800, 61000, 61240], "updated_at": 1751049600000 }
  ],
  "server_ts": 1751049605000
}
```

---

## 6. 顶部行情条（聚合便捷接口）

### `GET /api/v1/ticker`
跑马灯用的聚合快照（金银铜油 + 主要股指 + 美元 + 10Y + BTC）。响应 `data`：
```json
{
  "items": [
    { "key": "黄金", "value": "2356.4", "change_pct": 0.82 },
    { "key": "WTI", "value": "81.92", "change_pct": 2.31 }
  ],
  "server_ts": 1751049605000
}
```
> 前端映射：`k ← key`、`v ← value`、`c ← change_pct`。可由前端自行聚合，提供此接口是为省事。

---

## 7. 宏观信号

### `GET /api/v1/macro/hormuz`
霍尔木兹海峡通行量（源：Windward 公开页解析）。响应 `data`：
> `series` 仅含后台**真实观测到的日期**（随运行天数增长，初期较短，不伪造历史）；`change_pct` 用真实相邻观测日计算，不足两天时为 0。
```json
{
  "today": 92, "change_pct": -7.0, "unit": "艘/日",
  "series": [ { "date": "2026-06-01", "value": 100 }, { "date": "2026-06-02", "value": 98 } ],
  "source": "...", "updated_at": 1751049600000
}
```

### `GET /api/v1/macro/fedwatch`
下次 FOMC 利率路径概率（CME FedWatch）。响应 `data`：
> 后端优先读取 live 缓存；默认通过 `PERIPHERA_FEDWATCH_PROXY=http://127.0.0.1:33210` 抓取 Investing Fed Rate Monitor 免费页第一张会议卡，也可设置 `PERIPHERA_FEDWATCH_SOURCE=cme/json` 接 CME 或兼容 JSON。抓取失败时回退 seed，JSON 契约不变。刷新间隔 `PERIPHERA_FEDWATCH_INTERVAL` 默认 300s。
```json
{
  "meeting": "7月 FOMC",
  "meeting_date": "2026-07-29",
  "outcomes": [
    { "label": "维持不变", "prob": 58 },
    { "label": "降息25bp", "prob": 38 },
    { "label": "降息50bp", "prob": 4 }
  ],
  "updated_at": 1751049600000
}
```

---

## 8. 每日 AI 简报（P1，Coze 生成）

### `GET /api/v1/ai/briefing`
Query：`date`（默认今日，`YYYY-MM-DD`）。响应 `data`：
```json
{
  "date": "2026-06-27",
  "headline": "原油与避险金属同步走强……",
  "body": "隔夜美股科技与能源领涨……",
  "points": ["WTI +2.3% ……", "黄金创阶段新高 ……"],
  "tags": ["原油", "避险金属", "FedWatch"],
  "generated_at": 1751049600000
}
```
> 由后台 cron 调用**火山引擎方舟 Ark（豆包 Seed 模型）**生成并按 `date` 缓存（启动延迟 60s 预热、之后每 6h 重生成）；以当前实时行情+新闻头条为上下文。未配 `SEED_API_KEY` 或生成失败时回退 seed。结构不变。

---

## 9. 健康检查

### `GET /api/v1/health`
```json
{ "code": 0, "msg": "Success", "data": { "ok": true, "version": "1.3.4", "server_ts": 1751049605000 } }
```

---

## 附：模块 → 接口 映射速查

| 前端模块 | 接口 |
|----------|------|
| 顶部 ticker | `GET /ticker` |
| 实时新闻（/news、portal widget） | `GET /news`、`GET /news/sources` |
| 外资研报（/research、portal widget、录入页） | `GET /research`、`/research/:id`、`POST/PUT/DELETE /research` |
| 全球股市 | `GET /markets/indices` |
| 美股板块涨幅 | `GET /markets/us-sectors` |
| 金银铜油 / 暗金暗油 | `GET /commodities` |
| 原油实时行情 | `GET /commodities/crude` |
| 加密行情 | `GET /crypto` |
| 霍尔木兹通行量 | `GET /macro/hormuz` |
| FedWatch | `GET /macro/fedwatch` |
| 每日 AI 简报 | `GET /ai/briefing` |
