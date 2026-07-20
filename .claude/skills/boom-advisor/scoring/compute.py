#!/usr/bin/env python3
# 按五维标准计算评分并生成报告
# 输入 /tmp/scores_raw.json + /tmp/boom_doc.json + qualitative.py（定性表，人工维护）
# 输出 /tmp/scores_final.json + 仓库根目录 boom_scores_<date>.md
import json, os, sys
from datetime import datetime
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from qualitative import SECTOR_PRICING, STOCK_PRICING_OVERRIDE, SECTOR_GROWTH, Q, HK_MCAP

d = json.load(open('/tmp/scores_raw.json'))
doc = json.load(open('/tmp/boom_doc.json'))
sector_name = {s['id']: s['name'] for s in doc['sectors']}

def growth_score(pe, pred, sector_g):
    # PEG + 三年预期PE(增速按 g/0.6g/0.4g 衰减、单年封顶100%) + 行业增速
    if pe is None or pe <= 0 or pred is None:
        return None, {}
    total = 0; detail = {}
    if pred <= 0:
        detail['PEG'] = ('增速为负', -2); total -= 2; pe3 = pe
    else:
        peg = pe / pred
        s = 2 if peg < 0.8 else 1 if peg < 1.2 else 0 if peg < 2 else -1 if peg < 3 else -2
        detail['PEG'] = (round(peg, 2), s); total += s
        g1 = min(pred, 100) / 100
        pe3 = pe / ((1 + g1) * (1 + 0.6 * g1) * (1 + 0.4 * g1))
    s = 2 if pe3 < 15 else 1 if pe3 < 25 else 0 if pe3 < 35 else -1
    detail['三年预期PE'] = (round(pe3, 1), s); total += s
    s = 1 if sector_g > 30 else 0 if sector_g >= 10 else -1
    detail['行业增速'] = (sector_g, s); total += s
    label = ('显著低估' if total >= 4 else '合理偏低' if total >= 2 else '合理' if total >= 0
             else '偏贵' if total >= -2 else '透支')
    return total, {'label': label, **detail}

def dividend_score(pe, pe_pct):
    if pe is None:
        return None, {}
    total = 0; detail = {}
    s = 2 if 0 < pe < 10 else 1 if pe < 15 else 0 if pe < 25 else -1
    detail['PE'] = (round(pe, 1), s); total += s
    if pe_pct is not None:
        s = 2 if pe_pct < 30 else 1 if pe_pct < 50 else 0 if pe_pct < 70 else -1
        detail['PE分位'] = (round(pe_pct), s); total += s
    else:
        detail['PE分位'] = ('缺失', 0)
    label = '低估' if total >= 3 else '合理偏低' if total >= 1 else '合理' if total == 0 else '偏贵'
    return total, {'label': label, **detail}

def tech_score(r):
    if 'd_close' not in r:
        return None, None, {}
    c = r['d_close']; day = 0; week = 0; notes = []
    if r.get('d_ma20') and c > r['d_ma20']: day += 1
    if r.get('d_ma60') and c > r['d_ma60']: day += 1
    if r.get('d_ma20') and r.get('d_ma60') and r['d_ma20'] > r['d_ma60']: day += 1
    if r.get('d_off_high60') is not None and r['d_off_high60'] < 5: day += 1
    if r.get('d_macd') and r['d_macd']['golden']: day += 1
    if r.get('d_ma60') and c < r['d_ma60']: notes.append('破位MA60预警')
    if r.get('w_ma20') and c > r['w_ma20']: week += 2
    if r.get('w_ma30') and c > r['w_ma30']: week += 1
    if r.get('w_macd'):
        if r['w_macd']['golden']: week += 1
        if r['w_macd']['hist'] > 0 and r['w_macd']['hist'] > r['w_macd']['hist_prev']: week += 1
    if r.get('w_ma20') and c < r['w_ma20']: notes.append('跌破20周线')
    rsi = r.get('d_rsi14')
    if rsi is not None:
        if rsi > 80: notes.append(f'RSI{rsi:.0f}短期过热')
        elif rsi < 30: notes.append(f'RSI{rsi:.0f}超卖')
    total = day + week
    label = '强趋势' if total >= 8 else '趋势健康' if total >= 5 else '趋势走弱' if total >= 3 else '空头排列'
    return day, week, {'label': label, 'notes': notes}

def elasticity(r):
    total = 0; detail = []
    mcap = r.get('marketCap') or HK_MCAP.get(r['code'])
    if mcap is not None:
        if mcap < 200: total += 2; detail.append(f'市值{mcap:.0f}亿+2')
        elif mcap < 800: total += 1; detail.append(f'市值{mcap:.0f}亿+1')
        elif mcap > 2000: total -= 1; detail.append(f'市值{mcap:.0f}亿-1')
        else: detail.append(f'市值{mcap:.0f}亿+0')
    vol = r.get('ann_vol')
    if vol is not None:
        if vol > 60: total += 2; detail.append(f'波动{vol:.0f}%+2')
        elif vol > 40: total += 1; detail.append(f'波动{vol:.0f}%+1')
        elif vol < 30: total -= 1; detail.append(f'波动{vol:.0f}%-1')
        else: detail.append(f'波动{vol:.0f}%+0')
    if (r.get('pred') or 0) > 60: total += 1; detail.append('高预期增速+1')
    if r.get('role') == 'elastic': total += 1; detail.append('弹性仓+1')
    label = '高' if total >= 4 else '中' if total >= 2 else '低'
    return label, total, detail

def fmt_price(p):
    if p is None:
        return '—'
    return f"{p:.0f}" if p >= 100 else f"{p:.1f}" if p >= 10 else f"{p:.2f}"

def strategy(row, r):
    # 操作策略（确定性规则）。买入分左侧（护城河+估值）与右侧（趋势确认）；
    # 卖出触发：估值偏贵 / 短期涨幅过大(20日涨幅≥20%或RSI≥80) / 破20日线未收回(连续3日)
    price = r.get('d_close') or row.get('price')
    ma20, ma60, w20 = r.get('d_ma20'), r.get('d_ma60'), r.get('w_ma20')
    chg20, rsi = r.get('chg20d'), r.get('d_rsi14')
    closes25 = r.get('closes25') or []
    held = row['held']
    g, dv = row.get('growth'), row.get('dividend')
    no_valuation = g is None and dv is None
    cheap = bool(g and g['score'] >= 2) or bool(dv and dv['score'] >= 2)
    rich = bool(g and g['score'] <= -1) or bool(dv and dv['score'] <= -1)
    quality = (row.get('moat') or 0) >= 7 and row.get('certainty') in ('A', 'B')
    overheat = (chg20 is not None and chg20 >= 20) or (rsi is not None and rsi >= 80)
    if price is None or ma20 is None:
        return {'action': '观望', 'text': '行情数据缺失，暂不给出操作'}
    # 破20日线未收回：最近连续3日收盘都低于对应交易日的MA20
    below20_days = 0
    if len(closes25) >= 23:
        for k in (0, 1, 2):
            end = len(closes25) - k
            if closes25[end - 1] < sum(closes25[end - 20:end]) / 20:
                below20_days += 1
            else:
                break
    below20_confirmed = below20_days >= 3
    below20 = price < ma20
    deep_broken = bool(ma60 and price < ma60 and w20 and price < w20)
    # 左侧挂单价：现价下方约4%，若20周线介于其间则贴着20周线；暂停线取20周线或现价-8%
    left_ref = price * 0.96
    if w20 and left_ref < w20 < price:
        left_ref = w20
    stop_ref = w20 if (w20 and w20 < price) else price * 0.92
    hk_note = '（港股缺估值数据，仅按技术面与护城河）' if no_valuation and row.get('isHK') else ''

    if held:
        if row.get('cycle_pos') == '高位过热':
            return {'action': '卖出', 'text': '周期高位过热，卖出'}
        if rich and (overheat or below20_confirmed):
            why = '估值偏贵' + ('＋短期涨幅过大' if overheat else '') + ('＋破20日线未收回' if below20_confirmed else '')
            return {'action': '卖出', 'text': why + '，卖出'}
        if rich:
            return {'action': '减仓', 'text': '估值偏贵，反弹减仓一半'}
        if overheat and not cheap:
            return {'action': '减仓', 'text': f'近20日涨{chg20:.0f}%，短期涨幅过大，减仓一半'}
        if row.get('cycle_pos') == '右侧后段' and (below20_confirmed or deep_broken):
            return {'action': '减仓', 'text': '周期右侧后段＋技术破位，减仓一半'}
        if deep_broken and not (quality and cheap):
            return {'action': '卖出', 'text': f'破位未收回（60日线{fmt_price(ma60)}与20周线{fmt_price(w20)}下方），卖出{hk_note}'}
        if quality and cheap and (below20 or below20_confirmed or deep_broken):
            return {'action': '左侧加仓',
                    'text': f'护城河{row["moat"]}/10＋估值低估，回调即机会：{fmt_price(left_ref)}元左侧加仓3成，跌破{fmt_price(stop_ref)}元暂停加仓'}
        if price > ma20:
            return {'action': '持有',
                    'text': f'趋势健康，持有；回踩20日线（≈{fmt_price(ma20)}元）可加仓，连续3日收不回20日线则减仓{hk_note}'}
        return {'action': '持有',
                'text': f'持有；卖出线：连续3日收不回20日线（≈{fmt_price(ma20)}元）{hk_note}'}

    # 未持仓
    if rich or row.get('cycle_pos') == '高位过热':
        return {'action': '回避', 'text': '估值偏贵' + ('/周期高位' if row.get('cycle_pos') == '高位过热' else '') + '，回避'}
    if overheat:
        return {'action': '观望', 'text': f'近20日涨{chg20:.0f}%，短期涨幅过大，等回调企稳再看'}
    if quality and cheap and price > ma20 and not below20_confirmed:
        return {'action': '右侧买入',
                'text': f'企稳站上20日线，右侧买入3成；跌回20日线（≈{fmt_price(ma20)}元）止损'}
    if quality and cheap:
        return {'action': '左侧买入',
                'text': f'护城河{row["moat"]}/10＋估值低估，{fmt_price(left_ref)}元左侧建仓3成，跌破{fmt_price(stop_ref)}元暂停'}
    if cheap:
        return {'action': '观望', 'text': '估值偏低但护城河/确定性一般，站稳20日线再右侧介入'}
    return {'action': '观望', 'text': f'性价比不突出，观望{hk_note}'}

out = []
missing_q = []
for r in d['results']:
    code = r['code'].lower()
    q = Q.get(code)
    if not q:
        missing_q.append(f"{r['name']} {r['code']}")
    moat, cert, risk, cyc = q if q else (None, '?', '（定性表缺失，需人工补充 qualitative.py）', None)
    dims = STOCK_PRICING_OVERRIDE.get(code, SECTOR_PRICING.get(r['sectorId'], ['growth']))
    row = {
        'code': r['code'], 'name': r['name'], 'sector': sector_name.get(r['sectorId'], ''),
        'role': r.get('role'), 'held': (r.get('count') or 0) > 0, 'isHK': r.get('isHK'),
        'price': r.get('price'), 'pe': r.get('pe'), 'pred': r.get('pred'),
        'marketCap': r.get('marketCap') or HK_MCAP.get(r['code']),
        'pe_pct': r.get('pe_pct'), 'chg60d': r.get('chg60d'), 'kdate': r.get('kdate'),
        'moat': moat,
        'moat_label': None if moat is None else ('宽' if moat >= 8 else '中' if moat >= 5 else '弱' if moat >= 3 else '无'),
        'pricing_dims': dims, 'certainty': cert, 'risk': risk, 'cycle_pos': cyc,
    }
    if 'growth' in dims:
        gs, gd = growth_score(r.get('pe'), r.get('pred'), SECTOR_GROWTH.get(r['sectorId'], 15))
        row['growth'] = {'score': gs, **gd} if gs is not None else None
    if 'dividend' in dims:
        ds, dd = dividend_score(r.get('pe'), r.get('pe_pct'))
        row['dividend'] = {'score': ds, **dd} if ds is not None else None
    day, week, td = tech_score(r)
    row['tech'] = None if day is None else {'day': day, 'week': week, 'total': day + week, **td}
    el, es, ed = elasticity(r)
    row['elastic'] = {'label': el, 'score': es, 'detail': ed}
    row['strategy'] = strategy(row, r)
    out.append(row)

json.dump(out, open('/tmp/scores_final.json', 'w'), ensure_ascii=False, indent=1)

# 报告
def fmt_pricing(r):
    parts = []
    if r.get('growth'):
        g = r['growth']
        parts.append(f"成长{g['score']:+d}({g['label']},PEG {g.get('PEG',('—',))[0]},3yPE {g.get('三年预期PE',('—',))[0]})")
    if r.get('dividend'):
        dv = r['dividend']
        pct = dv.get('PE分位', ('—',))[0]
        parts.append(f"红利{dv['score']:+d}({dv['label']},PE {dv['PE'][0]},分位{pct}{'%' if pct != '缺失' else ''})")
    if r.get('cycle_pos'):
        parts.append(f"周期:{r['cycle_pos']}")
    return '；'.join(parts) if parts else '—（数据缺失）'

def fmt_tech(r):
    t = r.get('tech')
    if not t:
        return '—'
    s = f"日{t['day']}/5+周{t['week']}/5={t['total']}({t['label']})"
    if t['notes']:
        s += ' ⚠' + ','.join(t['notes'])
    return s

date = d.get('latest_date', '')
lines = [f"# 股票池五维评分报告（数据截至 {date} 收盘）\n",
         "评分标准见 scoring/README.md。定性分（护城河/确定性/风险/周期位置）来自人工维护的 qualitative.py。\n"]
by_sector = {}
for r in out:
    by_sector.setdefault(r['sector'], []).append(r)
for sec, items in by_sector.items():
    lines.append(f"\n## {sec}\n")
    items.sort(key=lambda x: (not x['held'], x['role'] != 'leader', -(x['moat'] or 0)))
    for r in items:
        held = '★持仓' if r['held'] else '观察'
        pe = f"{r['pe']:.0f}" if r['pe'] else '—'
        pred = f"{r['pred']:.0f}%" if r['pred'] is not None else '—'
        mc = f"{r['marketCap']:.0f}亿" if r['marketCap'] else '—'
        chg = f"，近60日{r['chg60d']:+.0f}%" if r.get('chg60d') is not None else ''
        lines.append(f"### {r['name']} {r['code']}（{r['role']}，{held}）")
        lines.append(f"- 概况：现价{r['price']}，市值{mc}，PE {pe}，预测增速{pred}{chg}")
        lines.append(f"- 护城河：{r['moat']}/10（{r['moat_label']}）")
        lines.append(f"- 定价：{fmt_pricing(r)}")
        lines.append(f"- 技术面：{fmt_tech(r)}")
        lines.append(f"- 弹性：{r['elastic']['label']}（{'，'.join(r['elastic']['detail'])}）")
        lines.append(f"- 确定性：{r['certainty']}级 | 最大挑战：{r['risk']}")
        lines.append(f"- **策略：{r['strategy']['action']}** — {r['strategy']['text']}")
        lines.append("")

repo_root = os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), '../../../..'))
report = os.path.join(repo_root, f"boom_scores_{date or 'latest'}.md")
open(report, 'w').write('\n'.join(lines))

# JSON 报告（后端 /api/boom/scores 列表与详情接口的数据源），同数据日期重跑覆盖
alerts = [{'name': x['name'], 'code': x['code'], 'notes': x['tech']['notes']}
          for x in out if x['held'] and x.get('tech') and x['tech']['notes']]
buy_actions = ('左侧买入', '右侧买入', '左侧加仓')
sell_actions = ('卖出', '减仓')
report_json = {
    'date': date or 'latest',
    'generatedAt': datetime.now().astimezone().isoformat(timespec='seconds'),
    'total': len(out),
    'held': sum(1 for x in out if x['held']),
    'buys': sum(1 for x in out if x['strategy']['action'] in buy_actions),
    'sells': sum(1 for x in out if x['strategy']['action'] in sell_actions),
    'alerts': alerts,
    'stocks': out,
}
scores_dir = os.path.join(repo_root, 'misc/data/boom_scores')
os.makedirs(scores_dir, exist_ok=True)
json_file = os.path.join(scores_dir, f"boom_scores_{date or 'latest'}.json")
tmp = json_file + '.tmp'
json.dump(report_json, open(tmp, 'w'), ensure_ascii=False)
os.replace(tmp, json_file)
print(f"scored {len(out)} stocks ({sum(1 for x in out if x['held'])} held)")
print(f"report md: {report}\nreport json: {json_file}")
if missing_q:
    print("定性表缺失（请在 qualitative.py 补充）:", '; '.join(missing_q))
for x in out:
    if x['held'] and x['tech'] and x['tech']['notes']:
        print('HELD-ALERT', x['name'], ','.join(x['tech']['notes']))
