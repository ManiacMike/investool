#!/usr/bin/env node
// 抖音视频/博主主页 → 豆包(专家模型)分析 → 结构化研报 JSON → 落 MySQL
//
// 两种模式:
//   单视频(位置参数或 --profile 传入含 modal_id / /video/<id> 的链接):只采集该视频
//   博主主页(--profile 传入 /user/ 主页链接):枚举主页最新 N 条视频逐条采集
//
// 用法:
//   node scripts/douyin_to_report.mjs "<抖音视频链接>"                    # 单视频,CLI 模式(POST 落库)
//   node scripts/douyin_to_report.mjs --profile "<主页或视频链接>" --limit 10
//   node scripts/douyin_to_report.mjs --profile "<链接>" --stream        # 流式模式(吐 NDJSON,不 POST,由调用方落库)
//
// 例:
//   node scripts/douyin_to_report.mjs "https://www.douyin.com/user/XXX?modal_id=7652965352200031538"
//   node scripts/douyin_to_report.mjs "https://www.douyin.com/video/7652965352200031538"
//   node scripts/douyin_to_report.mjs --profile "https://www.douyin.com/user/MS4wLjxxxx" --limit 5 --stream
//
// 前置条件:
//   - 外部 Chrome 以远程调试方式启动(默认端口 9222),且已登录抖音/豆包
//       /Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome --remote-debugging-port=9222
//   - CLI 模式下后台服务运行在 http://localhost:4869 (可用 API_BASE 覆盖),研报落 MySQL periphera 库
//   - 流式模式(--stream)不写后台,记录随 video_done 事件吐出,由 Go 调用方落库
//
// 参数 / 环境变量:
//   --profile <url>   博主主页链接或视频链接(与位置参数二选一)
//   --limit <n>       主页模式最多采集条数 (默认 10)
//   --stream          输出 NDJSON 事件到 stdout(人类日志转 stderr),且不 POST 后台
//   --skip-ids <csv>  逗号分隔的 video_id,枚举后在豆包分析前跳过(已入库去重)
//   CDP_PORT    Chrome 远程调试端口 (默认 9222)
//   API_BASE    后台地址 (默认 http://localhost:4869),CLI 模式写入 ${API_BASE}/api/v1/research/import
//   GEN_TIMEOUT 等待豆包生成的最长秒数 (默认 360)

const CDP_PORT = process.env.CDP_PORT || '9222';
const API_BASE = (process.env.API_BASE || 'http://localhost:4869').replace(/\/+$/, '');
const GEN_TIMEOUT = parseInt(process.env.GEN_TIMEOUT || '360', 10);

// ---- 参数解析 ----
function parseArgs(argv) {
  const opts = { profile: '', limit: 10, stream: false, skipIds: [], positional: '' };
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a === '--profile') opts.profile = argv[++i] || '';
    else if (a === '--limit') opts.limit = Math.max(1, parseInt(argv[++i] || '10', 10) || 10);
    else if (a === '--stream') opts.stream = true;
    else if (a === '--skip-ids') opts.skipIds = (argv[++i] || '').split(',').map(s => s.trim()).filter(Boolean);
    else if (!a.startsWith('--') && !opts.positional) opts.positional = a;
  }
  return opts;
}

const OPTS = parseArgs(process.argv.slice(2));
const STREAM = OPTS.stream;

const PROMPT_TEMPLATE = `请把这个视频分析整理成结构化 JSON 数组输出，每个视频对应一个对象。严格使用以下字段，值用中文，没有对应信息的填空字符串 ""。只输出 JSON，不要任何额外说明文字、不要代码块标记：
[{
  "publish_time": "研报时间",
  "industry_category": "行业分类",
  "institution_name": "机构名称",
  "research_target": "调研目标",
  "report_type": "研报类型",
  "core_content": "核心内容",
  "target_price": "目标价",
  "investment_rating": "投资评级",
  "rating_change": "评级变动",
  "earnings_forecast_adjustment": "盈利预测调整",
  "core_catalyst": "核心催化剂",
  "core_risk_warning": "核心风险提示",
  "author": "作者"
}]`;

// 调用方(页面)可通过环境变量覆盖 prompt；其中 {{video_id}} 为动态占位符，发送前替换为当前视频 ID。
const PROMPT_OVERRIDE = (process.env.PERIPHERA_DOUYIN_PROMPT || '').trim();

// buildPrompt 组装发给豆包的最终 prompt：
//   - 有覆盖 prompt：优先替换 {{video_id}} 占位符；若模板未含占位符则在末尾补上视频链接（兜底）。
//   - 无覆盖 prompt：用内置模板 + 视频链接（保持原行为）。
function buildPrompt(videoId, link) {
  if (PROMPT_OVERRIDE) {
    return PROMPT_OVERRIDE.includes('{{video_id}}')
      ? PROMPT_OVERRIDE.replaceAll('{{video_id}}', videoId)
      : PROMPT_OVERRIDE + '\n' + link;
  }
  return PROMPT_TEMPLATE + '\n' + link;
}

const sleep = (ms) => new Promise(r => setTimeout(r, ms));
// 人类日志:流式模式下走 stderr,避免污染 stdout 的 NDJSON 事件流
const log = (...a) => STREAM ? console.error('[douyin→report]', ...a) : console.log('[douyin→report]', ...a);
// NDJSON 事件:仅流式模式下向 stdout 输出一行 JSON
const emit = (obj) => { if (STREAM) process.stdout.write(JSON.stringify(obj) + '\n'); };

function extractVideoId(link) {
  const m = link.match(/modal_id=(\d+)/) || link.match(/\/video\/(\d+)/) || link.match(/(\d{15,})/);
  return m ? m[1] : null;
}

// 判断链接是否指向“具体某个视频”(含 modal_id 或 /video/<id>);否则视为博主主页
function isSingleVideoLink(link) {
  return /modal_id=\d+/.test(link) || /\/video\/\d+/.test(link);
}

// ---- 极简 CDP 客户端 (依赖 Node 18+ 的全局 WebSocket / fetch) ----
class CDP {
  constructor(ws) { this.ws = ws; this.id = 0; this.pend = new Map();
    ws.onmessage = (e) => { const d = JSON.parse(e.data); if (d.id && this.pend.has(d.id)) { this.pend.get(d.id)(d); this.pend.delete(d.id); } };
  }
  static async connect(port) {
    const ver = await (await fetch(`http://localhost:${port}/json/version`)).json();
    const ws = new WebSocket(ver.webSocketDebuggerUrl);
    await new Promise((res, rej) => { ws.onopen = res; ws.onerror = rej; });
    return new CDP(ws);
  }
  send(method, params = {}, sessionId) {
    return new Promise((res) => { const id = ++this.id; this.pend.set(id, res);
      this.ws.send(JSON.stringify(sessionId ? { id, method, params, sessionId } : { id, method, params })); });
  }
  async attach(targetId) {
    const r = await this.send('Target.attachToTarget', { targetId, flatten: true });
    const sid = r.result.sessionId;
    await this.send('Page.enable', {}, sid);
    await this.send('Runtime.enable', {}, sid);
    return sid;
  }
  async eval(sid, expression, awaitPromise = false) {
    const r = await this.send('Runtime.evaluate', { expression, returnByValue: true, awaitPromise, timeout: 30000 }, sid);
    if (r.result.exceptionDetails) throw new Error('eval: ' + JSON.stringify(r.result.exceptionDetails).slice(0, 300));
    return r.result.result.value;
  }
  async click(sid, x, y) {
    await this.send('Input.dispatchMouseEvent', { type: 'mouseMoved', x, y }, sid);
    await this.send('Input.dispatchMouseEvent', { type: 'mousePressed', x, y, button: 'left', clickCount: 1 }, sid);
    await this.send('Input.dispatchMouseEvent', { type: 'mouseReleased', x, y, button: 'left', clickCount: 1 }, sid);
  }
  async enter(sid) {
    const k = { key: 'Enter', code: 'Enter', windowsVirtualKeyCode: 13, nativeVirtualKeyCode: 13 };
    await this.send('Input.dispatchKeyEvent', { type: 'keyDown', ...k }, sid);
    await this.send('Input.dispatchKeyEvent', { type: 'keyUp', ...k }, sid);
  }
  // 关闭一个 tab(枚举/分析完清理,避免堆积)
  async closeTarget(targetId) { try { await this.send('Target.closeTarget', { targetId }); } catch {} }
}

// ---- 在豆包页面里找元素坐标 / 文本的工具表达式 ----
const findChipExpr = (label) => `(()=>{const b=[...document.querySelectorAll('button')].find(b=>(b.innerText||'').trim()===${JSON.stringify(label)});if(!b)return null;const r=b.getBoundingClientRect();return{x:Math.round(r.x+r.width/2),y:Math.round(r.y+r.height/2)};})()`;
const findLeafExpr = (label) => `(()=>{for(const e of document.querySelectorAll('*')){const own=[...e.childNodes].filter(n=>n.nodeType===3).map(n=>n.textContent.trim()).join('');if(own===${JSON.stringify(label)}){const r=e.getBoundingClientRect();return{x:Math.round(r.x+r.width/2),y:Math.round(r.y+r.height/2)};}}return null;})()`;
// 抓取“同时含 publish_time 与 author”的最大元素文本
const grabExpr = `(()=>{let best='';for(const el of document.querySelectorAll('*')){const t=el.innerText||'';if(t.includes('"publish_time"')&&t.includes('"author"')&&t.length>best.length)best=t;}return best;})()`;

// 从抖音主页页面 DOM 收集视频 id(链接锚点里的 /video/<id> 或 modal_id=<id>)
// best-effort:抖音前端结构可能变化,需对真实登录浏览器微调此表达式
const collectVideoIdsExpr = `(()=>{const ids=[];const seen=new Set();
  for(const a of document.querySelectorAll('a[href*="/video/"],a[href*="modal_id="]')){
    const h=a.getAttribute('href')||'';
    const m=h.match(/modal_id=(\\d+)/)||h.match(/\\/video\\/(\\d+)/);
    if(m&&!seen.has(m[1])){seen.add(m[1]);ids.push(m[1]);}
  }
  return ids;})()`;

// 抽取文本里所有“括号平衡”的顶层 JSON 数组片段。
function extractBalancedArrays(text) {
  const arrays = [];
  let depth = 0, start = -1;
  for (let i = 0; i < text.length; i++) {
    const c = text[i];
    if (c === '[') { if (depth === 0) start = i; depth++; }
    else if (c === ']') { if (depth > 0) { depth--; if (depth === 0 && start >= 0) { arrays.push(text.slice(start, i + 1)); start = -1; } } }
  }
  return arrays;
}

// 判断某对象是否是 prompt 里的“示例模板”回显(字段值仍等于字段名占位),用于兜底剔除。
function looksLikeTemplate(o) {
  return !!o && (o.publish_time === '研报时间' || o.institution_name === '机构名称'
    || o.research_target === '调研目标' || o.report_type === '研报类型' || o.author === '作者');
}

// 从豆包返回的整段文本里提取“真实回答”的研报数组。
// 关键:抓到的 DOM 文本里同时含“问题回显”(我们发出的 prompt,内含示例 JSON)和“回答”。
// 对话里问题在上、回答在下,且 prompt 末尾就是视频链接(sentinel);因此先切掉“最后一次出现
// sentinel 之前”的全部文本,把示例 JSON 连同问题一起丢弃,只在其后的“回答”里找数组。
// 找不到有效数组则返回 null —— 宁可让上层报错,也绝不回退到示例回显。
function parseReportsFromText(raw, sentinel) {
  let text = raw;
  if (sentinel) {
    const idx = raw.lastIndexOf(sentinel);
    if (idx >= 0) text = raw.slice(idx + sentinel.length);
  }
  const arrays = extractBalancedArrays(text);
  // 回答通常是其后最后一个对象数组;跳过任何仍是示例模板的数组。
  for (let i = arrays.length - 1; i >= 0; i--) {
    try {
      const d = JSON.parse(arrays[i]);
      if (Array.isArray(d) && d.length && typeof d[0] === 'object' && !looksLikeTemplate(d[0])) return d;
    } catch {}
  }
  return null;
}

// POST 到后台 /api/v1/research/import（后端按 video_id 去重 upsert，落 MySQL）
async function postToBackend(records) {
  const resp = await fetch(`${API_BASE}/api/v1/research/import`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ items: records }),
  });
  let body = {};
  try { body = await resp.json(); } catch {}
  if (!resp.ok || (typeof body.code === 'number' && body.code !== 0)) {
    throw new Error(`后台写入失败 HTTP ${resp.status}: ${JSON.stringify(body).slice(0, 200)}`);
  }
  return body.data || {};
}

// ---- 单视频流水线:建 tab → 选专家模型 → 发 prompt → 等生成 → 解析,返回 records ----
async function processVideo(cdp, videoId) {
  const link = `https://www.douyin.com/video/${videoId}`;

  // 1) 新建豆包 tab(防幻觉:每次独立对话)
  log('新建豆包对话 tab ...');
  const created = await cdp.send('Target.createTarget', { url: 'https://www.doubao.com/chat/' });
  const doubaoTarget = created.result.targetId;
  const sid = await cdp.attach(doubaoTarget);

  try {
    // 2) 等输入框就绪
    for (let i = 0; i < 30; i++) { if (await cdp.eval(sid, `!!document.querySelector('textarea')`)) break; await sleep(500); }
    if (!(await cdp.eval(sid, `!!document.querySelector('textarea')`))) throw new Error('豆包输入框未就绪(可能未登录)');

    // 3) 选择“专家”模型(非“快速”)
    log('切换到专家模型 ...');
    let ok = false;
    for (let attempt = 0; attempt < 2 && !ok; attempt++) {
      const chip = await cdp.eval(sid, findChipExpr('快速'));
      if (chip) { await cdp.click(sid, chip.x, chip.y); await sleep(700); }
      const exp = await cdp.eval(sid, findLeafExpr('专家'));
      if (exp) { await cdp.click(sid, exp.x, exp.y); await sleep(600); }
      const active = await cdp.eval(sid, `(()=>{const b=[...document.querySelectorAll('button')].find(b=>['快速','专家','办公任务'].includes((b.innerText||'').trim()));return b?b.innerText.trim():''})()`);
      ok = active === '专家';
      if (!ok) log('  专家未选中(当前:' + active + '),重试…');
    }
    if (!ok) log('⚠️ 未能确认专家模型,继续(请人工核对)');
    else log('  ✓ 专家模型已选中');

    // 4) 填入 prompt + 链接并发送
    log('发送分析指令 ...');
    const msg = buildPrompt(videoId, link);
    await cdp.eval(sid, `(()=>{const ta=document.querySelector('textarea');ta.focus();const s=Object.getOwnPropertyDescriptor(window.HTMLTextAreaElement.prototype,'value').set;s.call(ta,${JSON.stringify(msg)});ta.dispatchEvent(new Event('input',{bubbles:true}));return true;})()`);
    await sleep(300);
    await cdp.enter(sid);

    // 5) 等待生成稳定(长度连续 3 次不变 或 超时)
    log('等待豆包生成(最长 ' + GEN_TIMEOUT + 's)…');
    const deadline = Date.now() + GEN_TIMEOUT * 1000;
    let prev = -1, stable = 0, len = 0;
    while (Date.now() < deadline && stable < 3) {
      await sleep(5000);
      try { len = (await cdp.eval(sid, grabExpr) || '').length; } catch { len = 0; }
      if (len > 100 && len === prev) stable++; else stable = 0;
      if (!STREAM) process.stdout.write(`  len=${len} stable=${stable}\r`);
      prev = len;
    }
    if (!STREAM) console.log('');
    if (stable < 3) log('⚠️ 生成未在超时内稳定,尝试用当前内容解析');

    // 6) 提取并解析
    const raw = await cdp.eval(sid, grabExpr) || '';
    const records = parseReportsFromText(raw, link); // link 作 sentinel:切掉问题回显(含示例 JSON)
    if (!records || !records.length) {
      throw new Error('未能从豆包回复解析出 JSON。原始前 300 字:' + raw.slice(0, 300));
    }
    // 7) 附加 video_id / source_url
    for (const r of records) { r.video_id = videoId; r.source_url = link; }
    return records;
  } finally {
    await cdp.closeTarget(doubaoTarget);
  }
}

// ---- 博主主页枚举:打开主页 tab,滚动懒加载,收集视频 id,取前 limit 条 ----
async function enumerateProfileVideos(cdp, profileUrl, limit) {
  log('打开博主主页枚举视频 ...');
  const created = await cdp.send('Target.createTarget', { url: profileUrl });
  const target = created.result.targetId;
  const sid = await cdp.attach(target);
  try {
    // 等待首屏视频锚点出现
    let ids = [];
    for (let i = 0; i < 20; i++) {
      await sleep(1000);
      ids = await cdp.eval(sid, collectVideoIdsExpr) || [];
      if (ids.length > 0) break;
    }
    // 滚动加载更多,直到达到 limit 或连续多次无新增
    let stale = 0;
    while (ids.length < limit && stale < 5) {
      await cdp.eval(sid, `window.scrollTo(0, document.body.scrollHeight)`);
      await sleep(1500);
      const next = await cdp.eval(sid, collectVideoIdsExpr) || [];
      if (next.length > ids.length) { ids = next; stale = 0; } else { stale++; }
    }
    return ids.slice(0, limit);
  } finally {
    await cdp.closeTarget(target);
  }
}

async function main() {
  // 输入链接:优先 --profile,回退位置参数
  const input = (OPTS.profile || OPTS.positional || '').trim();
  if (!input) {
    console.error('用法: node scripts/douyin_to_report.mjs "<抖音视频链接>"  或  --profile "<主页/视频链接>" [--limit N] [--stream]');
    process.exit(1);
  }

  const skip = new Set(OPTS.skipIds);
  let cdp;
  try {
    emit({ type: 'stage', stage: 'connect', msg: '连接 Chrome 远程调试 ...' });
    cdp = await CDP.connect(CDP_PORT);
  } catch (e) {
    const msg = 'CDP 连接失败(Chrome 未开远程调试端口 ' + CDP_PORT + ' 或未启动)';
    emit({ type: 'error', error: msg });
    console.error('出错:', msg, '-', e.message || e);
    process.exit(1);
  }

  // 1) 确定要处理的 video_id 列表
  let videoIds = [];
  if (isSingleVideoLink(input)) {
    const vid = extractVideoId(input);
    if (!vid) { emit({ type: 'error', error: '无法解析视频ID: ' + input }); console.error('无法解析视频ID:', input); process.exit(1); }
    videoIds = [vid];
    emit({ type: 'list_done', total: 1, video_ids: videoIds });
  } else {
    emit({ type: 'stage', stage: 'list', msg: '枚举博主主页视频 ...' });
    try {
      videoIds = await enumerateProfileVideos(cdp, input, OPTS.limit);
    } catch (e) {
      emit({ type: 'error', error: '主页枚举失败: ' + (e.message || e) });
      console.error('主页枚举失败:', e.message || e); cdp.ws.close(); process.exit(2);
    }
    if (!videoIds.length) {
      emit({ type: 'error', error: '未在主页枚举到任何视频(可能未登录/结构变化/链接非主页)' });
      log('未枚举到视频'); cdp.ws.close(); process.exit(2);
    }
    emit({ type: 'list_done', total: videoIds.length, video_ids: videoIds });
    log('枚举到 ' + videoIds.length + ' 条视频');
  }

  // 2) 逐条处理(跳过已入库)
  const total = videoIds.length;
  let processed = 0, skipped = 0, failed = 0;
  const cliRecords = []; // CLI 模式累计后统一 POST

  for (let i = 0; i < total; i++) {
    const vid = videoIds[i];
    const idx = i + 1;
    if (skip.has(vid)) {
      skipped++;
      emit({ type: 'video_skip', video_id: vid, index: idx, total });
      log(`[${idx}/${total}] 跳过已采集 ${vid}`);
      continue;
    }
    emit({ type: 'video_start', video_id: vid, index: idx, total });
    log(`[${idx}/${total}] 采集 ${vid} ...`);
    try {
      const records = await processVideo(cdp, vid);
      processed++;
      emit({ type: 'video_done', video_id: vid, index: idx, total, records });
      if (!STREAM) cliRecords.push(...records);
      for (const r of records) log(`   • [${vid}] ${r.institution_name} | ${r.industry_category} | ${r.research_target}`);
    } catch (e) {
      failed++;
      emit({ type: 'video_error', video_id: vid, index: idx, total, error: e.message || String(e) });
      log(`   ✗ [${vid}] 失败: ${e.message || e}`);
    }
  }

  // 3) CLI 模式统一 POST 落库(流式模式由 Go 调用方落库)
  if (!STREAM && cliRecords.length) {
    log('写入后台 ...');
    try {
      const res = await postToBackend(cliRecords);
      log(`✓ 完成: 新增 ${res.added ?? '?'} 条, 更新 ${res.updated ?? '?'} 条`);
    } catch (e) {
      log('后台写入失败: ' + (e.message || e));
    }
  }

  emit({ type: 'done', processed, skipped, failed, total });
  log(`完成: 处理 ${processed} / 跳过 ${skipped} / 失败 ${failed} / 共 ${total}`);
  cdp.ws.close();
  process.exit(0);
}

main().catch((e) => {
  emit({ type: 'error', error: e.message || String(e) });
  console.error('出错:', e.message || e);
  process.exit(1);
});
