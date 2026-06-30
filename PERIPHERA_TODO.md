# PERIPHERA 数据接入 TODO（前端 + 后端）

> 契约见 [`PERIPHERA_API.md`](./PERIPHERA_API.md)。**契约即边界**：前后端各自按契约推进，不必互相等待。
> Owner 标记：**[BE]** = Claude（后端）｜**[FE]** = Codex（前端）。
> 决策：传输=轮询REST；行情源=扩展 sina/qq；新闻/宏观=后端接真实源；存储=按板块分别定（行情/新闻内存缓存+cron；**研报=MySQL `periphera` 库，已建库建表**）。
> ⚠️ 数据库凭据（localhost root）**不写入被 git 跟踪的 config.toml**，走 `config.prod.toml`（已 gitignore）或环境变量 `MYSQL_LOCALHOST_PASSWORD`。

状态：`[ ]` 待办 · `[~]` 进行中 · `[x]` 完成

---

## M0 · 接口骨架 & 前端数据层（解除互相阻塞）

目标：所有 `/api/v1/*` 端点先以**契约合法的数据**上线（真实源未就绪的板块返回 seed/占位但结构完全正确），前端据此搭好轮询层。

### 后端 [BE]
- [x] 新建 `api/periphera.go` + 在 `routes/routes.go` 注册 `/api/v1` 路由组
- [x] 复用 `routes/response` 信封；统一 `since/limit/codes/source/rating` 参数解析工具
- [x] `GET /api/v1/health`
- [x] 各端点返回 seed 数据（结构 100% 符合契约，已 curl 验证全部 200）：news / news/sources / research / research/:id / markets/indices / markets/us-sectors / commodities / commodities/crude / crypto / ticker / macro/hormuz / macro/fedwatch / ai/briefing
- [x] 数据/类型/seed 落在 `datacenter/periphera`（types + seed），api 层只做参数解析与封装
- [ ] 内存缓存层 `datacenter/periphera/cache`（带 TTL + 读写锁）—— 留到 M1 真实源接入时再加（seed 当前无状态）
- [x] CORS：前后端同域，无需额外处理（已确认）

### 前端 [FE]
- [x] 抽一个 API 客户端 `apiGet(path, params)`，解析 `{code,msg,data}`，`code!=0` 抛错
- [x] 抽一个轮询 composable（`usePolling(fetcher, intervalMs)`，页面隐藏时暂停 `visibilitychange`）
- [x] 全局开关 `USE_BACKEND`（true=走接口，false=回退现有 mock），便于联调期切换
- [x] 各模块的 `setInterval` mock → 替换为对应接口轮询（间隔见契约 0.7）
- [x] 新闻/研报红点：用 `since=上次 latest_ts` 增量拉取，新条目计入未读
- [x] 加载态/错误态：骨架屏 + 失败时保留上一帧数据，不空白
- [x] 看板定制/布局仍走 localStorage（不进后端）

---

## M1 · 行情真实源（sina/qq）

### 后端 [BE]
- [x] `datacenter/sina/realtime.go`：`hq.sinajs.cn` 实时行情拉取（带 Referer，免 GBK 解码，只取 ASCII 数字字段）
- [x] 金 `hf_GC`、银 `hf_SI`、铜 `hf_HG`(美分/磅)、WTI `hf_CL`、Brent `hf_OIL`
- [x] 股指：标普 `int_sp500`、纳指 `int_nasdaq`、道指 `int_dji`、日经 `int_nikkei`；**台股 `znb_TWSE`、韩 KOSPI `znb_KOSPI`**（已验证可用）
- [x] 加密 BTC `hf_BTC`（真实）；**ETH 无 sina 源 → 暂回退 seed**（待找 ETH 源）
- [x] 后台 3s 刷新（惰性+启动即开 `StartLive()`）写入内存缓存 `datacenter/periphera/live.go`；每 symbol 维护价格滚动窗口（sparkline / 原油分时）
- [x] 实装 `/commodities`、`/commodities/crude`、`/markets/indices`、`/crypto`、`/ticker`（live 优先，sina 失败/未就绪回退 seed）
- [ ] `/markets/us-sectors` 真实源（美股板块隔夜涨幅，源待定）—— 仍为 seed
- [~] **暗金/暗油**：当前＝连续电子盘（hf_ 本就是近 24h 全球盘），与日盘同源同值；真实独立「夜盘 vs 日盘」口径待补（见 Open Q6）
- 备注：台/韩 znb_ 为延迟数据，`is_open` 暂固定 true

### 前端 [FE]
- [x] 商品/股指/加密卡片绑定真实 `price/change_pct/spark`，价格跳动闪色用真实变化触发
- [x] 原油分时图绑定 `wti_series/brent_series`
- [x] 顶部 ticker 绑定 `/ticker`

---

## M2 · 新闻 & 研报

### 实时新闻 [BE]
- [x] **X/Twitter（马斯克 `elonmusk` + 白毛女神 `aleabitoreddit`）→ twscrape 自建免费方案**（已 E2E 跑通：musk 29 条 + 白毛 24 条进 `/news`）
  - [x] Python 旁路脚本 `scripts/twitter_fetch.py`（venv `scripts/venv`，`twscrape 0.19.1`）：cookie 自读项目根 `.env`，拉两账号 `user_tweets`，输出统一 JSON 到 stdout
  - [x] 账号池：`x_auth_token`+`x_ct0` cookie（在 `.env`，已 gitignore）初始化 twscrape SQLite 账号库 `scripts/.twscrape_accounts.db`
  - [x] Go cron `datacenter/periphera/news_live.go` 通过 `os/exec` 定时（默认 300s）调脚本 → 解析 JSON → 灌内存环形缓存（~200 条，按 published_at 倒序）；失败保留上次缓存
  - [x] **代理**：国内 x.com 必须走代理，`.env` 配 `X_PROXY=http://127.0.0.1:33210`；脚本已 monkeypatch 修复 twscrape XClIdGen 不透传 proxy 的 bug
  - [ ] 风险预案：doc_id 轮换/限频/封号 → 退避重试 + 账号轮换（当前单账号；cookie 轮换＝删 `scripts/.twscrape_accounts.db` 重建）
- [x] 路透/彭博 → **Google News RSS**（`news_rss.go`，E2E 跑通 reuters 33 / bloomberg 86 条）。⚠️ news.google.com 国内被墙，复用 `X_PROXY` 代理（仅 RSS 走代理，Binance/Windward 直连）
- [x] 归一化为契约结构，按 `tweet_id`（`x_<id>`）去重，环形缓冲（近 ~200 条）
- [x] cron 定时抓取；`/news` 支持 `since/source/limit`；`/news/sources` 真实计数
> 注：twscrape 引入 **Python 依赖**（旁路脚本，独立 venv），其余后端仍是 Go。
> 运行环境变量（均可选，见 `news_live.go` 顶部注释）：`PERIPHERA_TWITTER_{PYTHON,SCRIPT,TARGETS,LIMIT,INTERVAL}`。

### 实时新闻 [FE]
- [x] /news 页与 portal 新闻 widget 接 `/news`；"↑ N 条新推送"基于 `since` 增量
- [x] 来源筛选接 `/news/sources`

### 外资研报（持久化 + 录入页迁移）[BE]
- [x] 存储拍板：**MySQL `periphera.research_reports`**（已建库建表；补充列 author/earnings_forecast_adjustment/source_url）
- [x] DAO 层 `datacenter/periphera/{db.go,research_dao.go}`：`database/sql`+`go-sql-driver/mysql`，DSN 走 env（`PERIPHERA_MYSQL_PASSWORD`/`PERIPHERA_MYSQL_DSN`），不入 config.toml
- [x] `GET /research`、`/research/:id`（筛选 `rating/institution/q` + `summary` 统计；库不可用降级 seed）
- [x] `POST/PUT/DELETE /research` + `POST /research/import`（按 `dedup_key` upsert，全部 curl 验证）
- [x] **抖音→豆包脚本接入**：`scripts/douyin_to_report.mjs` 改为 POST 到 `/api/v1/research/import`（落 MySQL，video_id 去重），已 E2E 验证
- [ ] （可选）一次性迁移：把旧前端 localStorage 里的研报推到后端

### 外资研报 [FE]
- [x] /research 页与 portal 研报 widget 改为读 `/research`
- [x] `/invest/zen-research-report` 录入页：增删改查改走后端接口（后端写接口未上线时自动回退 localStorage）

---

## M3 · 宏观 & AI 简报

### 后端 [BE]
- [x] 霍尔木兹通行量 `/macro/hormuz`：解析 **Windward 公开页**（`hormuz_live.go`，15min 刷新），E2E 跑通（today=42=28进+14出）。序列只累积**真实观测日**（不伪造），change_pct 用真实相邻日；Windward 句式每日变，正则需随之维护
- [ ] FedWatch `/macro/fedwatch`：CME FedWatch 概率（抓取/接口），cron 刷新
- [x] AI 简报 `/ai/briefing`：**改用火山引擎方舟 Ark（豆包 Seed 模型，非 Coze）** 生成，已 E2E 跑通
  - `datacenter/periphera/briefing_live.go`：Ark OpenAI 兼容 `chat/completions`，以当前 live 行情+新闻头条为上下文，产出结构化 JSON（headline/body/points/tags），按 `date` 缓存；启动后延迟 60s 预热生成，之后每 6h 重生成；失败/未配 key 自动回退 `SeedBriefing`
  - 凭据走 `.env`（新增 Go 侧 `LoadDotEnv`，真实 env 优先）：`SEED_API_KEY` / `SEED_BASE_URL`(默认 `https://ark.cn-beijing.volces.com`) / `SEED_MODEL`(默认 `doubao-seed-1-6-250615`) / `PERIPHERA_BRIEFING_INTERVAL`(默认 21600s)
  - 注：`datacenter/coze` 仅用于选股打分，与简报无关，保持不动

### 前端 [FE]
- [x] 霍尔木兹 / FedWatch 图表绑定真实数据
- [x] （若启用）AI 简报模块/区块接 `/ai/briefing`

---

## 横切项（贯穿各阶段）
- [ ] [BE] 各上游源失败的降级：返回上次缓存 + 标 `stale`，绝不 5xx 打挂前端
- [ ] [BE] 配置项集中到 `config.toml`（各源开关/间隔/key）
- [ ] [BE] 关键接口加单测（解析/去重/缓存）
- [x] [FE] 全模块 loading/empty/error 三态
- [x] [FE/BE] 端到端冒烟：portal/news/research 三页在真实接口下跑通
- [ ] [BE] 更新 `PERIPHERA_API.md`（接口若有变动，契约先行）

---

## 数据库（已就绪）

MySQL `periphera`（utf8mb4），表 `research_reports`：
```
id VARCHAR(40) PK            -- 形如 r_xxxxxx
dedup_key VARCHAR(160) UNIQUE -- video_id 或 target|time|institution
publish_time, industry_category, institution_name, research_target,
report_type, target_price, investment_rating, rating_change VARCHAR
core_content, core_catalyst, core_risk_warning TEXT
video_id VARCHAR(64)
created_at, updated_at BIGINT  -- unix ms
索引: idx_created / idx_rating / idx_institution / idx_target
```
> 凭据：localhost root，密码走 env/`config.prod.toml`，**勿提交到 config.toml**。

---

## Open Questions（需你拍板，后端开工前确认）

1. ~~白毛股神/马斯克 数据源~~ ✅ **已完成**：X via twscrape，`elonmusk` + `aleabitoreddit`（白毛女神）。cookie 已写入 `.env`（`x_auth_token`/`x_ct0`），代理 `X_PROXY` 已配，E2E 跑通。
   - ⚠️ 当前单账号，无小号轮换；若被限频/封号需补充第二账号 cookie。
2. **路透/彭博 合规**：用官方 RSS / 第三方聚合 / 抓取？是否有可用 API key？（避免版权/反爬风险）
4. ~~**研报存储**：JSON 文件 vs MySQL？~~ ✅ 已定：MySQL `periphera`（库表已建）。
5. **霍尔木兹通行量**：有没有指定数据源（如 TankerTrackers / 自有数据）？否则我找公开近似源并注明口径。
6. **暗金/暗油**：你期望的"暗盘"口径＝COMEX/NYMEX 电子盘夜盘价，还是其他？决定我接哪个字段。
