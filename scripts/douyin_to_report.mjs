#!/usr/bin/env node
// 抖音视频 → 豆包(专家模型)分析 → 结构化研报 JSON → 写入研报页面 localStorage
//
// 用法:
//   node scripts/douyin_to_report.mjs "<抖音视频链接>"
//
// 例:
//   node scripts/douyin_to_report.mjs "https://www.douyin.com/user/XXX?modal_id=7652965352200031538"
//   node scripts/douyin_to_report.mjs "https://www.douyin.com/video/7652965352200031538"
//
// 前置条件:
//   - 外部 Chrome 以远程调试方式启动(默认端口 9222),且已登录抖音/豆包
//       /Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome --remote-debugging-port=9222
//   - 研报服务运行在 http://localhost:4869 (可用 REPORT_URL 覆盖)
//
// 环境变量:
//   CDP_PORT    Chrome 远程调试端口 (默认 9222)
//   REPORT_URL  研报页面地址 (默认 http://localhost:4869/invest/zen-research-report)
//   GEN_TIMEOUT 等待豆包生成的最长秒数 (默认 360)

const CDP_PORT = process.env.CDP_PORT || '9222';
const REPORT_URL = process.env.REPORT_URL || 'http://localhost:4869/invest/zen-research-report';
const GEN_TIMEOUT = parseInt(process.env.GEN_TIMEOUT || '360', 10);

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

const FIELDS = ['publish_time','industry_category','institution_name','research_target','report_type',
  'core_content','target_price','investment_rating','rating_change','earnings_forecast_adjustment',
  'core_catalyst','core_risk_warning','author'];

const sleep = (ms) => new Promise(r => setTimeout(r, ms));
const log = (...a) => console.log('[douyin→report]', ...a);

function extractVideoId(link) {
  const m = link.match(/modal_id=(\d+)/) || link.match(/\/video\/(\d+)/) || link.match(/(\d{15,})/);
  return m ? m[1] : null;
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
}

// ---- 在豆包页面里找元素坐标 / 文本的工具表达式 ----
const findChipExpr = (label) => `(()=>{const b=[...document.querySelectorAll('button')].find(b=>(b.innerText||'').trim()===${JSON.stringify(label)});if(!b)return null;const r=b.getBoundingClientRect();return{x:Math.round(r.x+r.width/2),y:Math.round(r.y+r.height/2)};})()`;
const findLeafExpr = (label) => `(()=>{for(const e of document.querySelectorAll('*')){const own=[...e.childNodes].filter(n=>n.nodeType===3).map(n=>n.textContent.trim()).join('');if(own===${JSON.stringify(label)}){const r=e.getBoundingClientRect();return{x:Math.round(r.x+r.width/2),y:Math.round(r.y+r.height/2)};}}return null;})()`;
// 抓取“同时含 publish_time 与 author”的最大元素文本
const grabExpr = `(()=>{let best='';for(const el of document.querySelectorAll('*')){const t=el.innerText||'';if(t.includes('"publish_time"')&&t.includes('"author"')&&t.length>best.length)best=t;}return best;})()`;

// 从豆包返回的整段文本里提取“真实”那个 JSON 数组(跳过 prompt 模板回显)
function parseReportsFromText(raw) {
  const arrays = [];
  let depth = 0, start = -1;
  for (let i = 0; i < raw.length; i++) {
    const c = raw[i];
    if (c === '[') { if (depth === 0) start = i; depth++; }
    else if (c === ']') { depth--; if (depth === 0 && start >= 0) { arrays.push(raw.slice(start, i + 1)); start = -1; } }
  }
  let chosen = null;
  for (const a of arrays) {
    try { const d = JSON.parse(a);
      if (Array.isArray(d) && d.length && typeof d[0] === 'object' && d[0].publish_time && d[0].publish_time !== '研报时间') { chosen = d; break; }
    } catch {}
  }
  if (!chosen) { // 回退:最后一个可解析的非空数组
    for (let i = arrays.length - 1; i >= 0; i--) { try { const d = JSON.parse(arrays[i]); if (Array.isArray(d) && d.length) { chosen = d; break; } } catch {} }
  }
  return chosen;
}

// 注入研报页面的 upsert(以 video_id 为唯一 key,存在则更新)
function importExpr(records) {
  return `(()=>{
    const incoming=${JSON.stringify(records)};
    const FIELDS=${JSON.stringify(FIELDS)};
    let reports=[]; try{reports=JSON.parse(localStorage.getItem('zen_research_reports')||'[]')}catch(e){reports=[]}
    const gid=()=>Date.now().toString(36)+Math.random().toString(36).slice(2);
    let added=0,updated=0;
    for(const item of incoming){
      if(!item||typeof item!=='object')continue;
      const r={id:item.id||gid(),video_id:item.video_id||''};
      for(const f of FIELDS) r[f]=item[f]||'';
      if(r.video_id){
        const i=reports.findIndex(x=>x.video_id===r.video_id);
        if(i>-1){r.id=reports[i].id;reports[i]=r;updated++;}else{reports.push(r);added++;}
      }else{
        const ex=reports.some(x=>x.research_target===r.research_target&&x.publish_time===r.publish_time&&x.institution_name===r.institution_name);
        if(!ex){reports.push(r);added++;}
      }
    }
    localStorage.setItem('zen_research_reports',JSON.stringify(reports));
    if(typeof loadReports==='function')loadReports();
    if(typeof renderTable==='function')renderTable();
    return {added,updated,total:reports.length};
  })()`;
}

async function main() {
  const link = process.argv[2];
  if (!link) { console.error('用法: node scripts/douyin_to_report.mjs "<抖音视频链接>"'); process.exit(1); }
  const videoId = extractVideoId(link);
  if (!videoId) { console.error('无法从链接中解析视频ID(需含 modal_id= 或 /video/<id>):', link); process.exit(1); }
  log('视频ID:', videoId);

  const cdp = await CDP.connect(CDP_PORT);

  // 1) 新建豆包 tab(防幻觉:每次独立对话)
  log('新建豆包对话 tab ...');
  const created = await cdp.send('Target.createTarget', { url: 'https://www.doubao.com/chat/' });
  const doubaoTarget = created.result.targetId;
  const sid = await cdp.attach(doubaoTarget);

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
  const msg = PROMPT_TEMPLATE + '\n' + link;
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
    process.stdout.write(`  len=${len} stable=${stable}\r`);
    prev = len;
  }
  console.log('');
  if (stable < 3) log('⚠️ 生成未在超时内稳定,尝试用当前内容解析');

  // 6) 提取并解析
  const raw = await cdp.eval(sid, grabExpr) || '';
  const records = parseReportsFromText(raw);
  if (!records || !records.length) { console.error('未能从豆包回复解析出 JSON。原始文本前 600 字:\n' + raw.slice(0, 600)); process.exit(2); }
  log('解析出 ' + records.length + ' 条记录');
  // 7) 附加 video_id / source_url
  for (const r of records) { r.video_id = videoId; r.source_url = link; }

  // 8) 写入研报页面(video_id 去重 upsert)
  log('写入研报页面 ...');
  const targets = (await cdp.send('Target.getTargets')).result.targetInfos.filter(t => t.type === 'page');
  let reportTarget = targets.find(t => t.url.includes('zen-research-report'));
  if (!reportTarget) {
    const c = await cdp.send('Target.createTarget', { url: REPORT_URL });
    reportTarget = { targetId: c.result.targetId };
    await sleep(3500);
  }
  const rsid = await cdp.attach(reportTarget.targetId);
  const res = await cdp.eval(rsid, importExpr(records));
  log(`✓ 完成: 新增 ${res.added} 条, 更新 ${res.updated} 条, 页面共 ${res.total} 条`);

  for (const r of records) log(`   • [${r.video_id}] ${r.institution_name} | ${r.industry_category} | ${r.research_target}`);
  cdp.ws.close();
  process.exit(0);
}

main().catch((e) => { console.error('出错:', e.message || e); process.exit(1); });
