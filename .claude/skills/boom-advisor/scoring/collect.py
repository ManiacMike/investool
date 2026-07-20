#!/usr/bin/env python3
# 采集评分所需数据 → /tmp/scores_raw.json
# 数据源：本地 investool 服务(股票池+基本面)、腾讯行情(日/周K线，东财限流时的稳定源)、东财emfront(PE历史分位)
import json, math, time, subprocess, sys, urllib.request

BASE = "http://localhost:4869"
OUT = "/tmp/scores_raw.json"

def local_json(path, data=None):
    if data is not None:
        req = urllib.request.Request(BASE + path, json.dumps(data).encode(),
                                     {'Content-Type': 'application/json'})
    else:
        req = urllib.request.Request(BASE + path)
    with urllib.request.urlopen(req, timeout=30) as r:
        return json.loads(r.read())

def curl_json(url):
    for _ in range(3):
        try:
            out = subprocess.run(['curl', '-s', '--max-time', '20', url],
                                 capture_output=True, text=True).stdout
            if out:
                return json.loads(out)
        except Exception:
            pass
        time.sleep(1)
    print(f"FAIL {url[:100]}", file=sys.stderr)
    return None

def hk_code(code):  # 池内港股代码存储为 "后两位.前三位"，如 988.09 = 09988
    a, b = code.split('.')
    return (b + a).zfill(5)

def tsymbol(st):
    if st.get('isHK'):
        return 'hk' + hk_code(st['code'])
    c6, mkt = st['code'].split('.')
    return mkt.lower() + c6

def kline(sym, period, n):
    url = f"https://web.ifzq.gtimg.cn/appstock/app/fqkline/get?param={sym},{period},,,{n},qfq"
    resp = curl_json(url)
    try:
        node = resp['data'][sym]
        arr = node.get('qfq' + period) or node.get(period) or []
        return [{'date': k[0], 'close': float(k[2]), 'high': float(k[3]), 'low': float(k[4])} for k in arr]
    except Exception:
        return []

def ma(vals, n):
    return sum(vals[-n:]) / n if len(vals) >= n else None

def ema_series(vals, n):
    a = 2.0 / (n + 1); out = [vals[0]]
    for v in vals[1:]:
        out.append(a * v + (1 - a) * out[-1])
    return out

def macd_state(closes):
    if len(closes) < 30:
        return None
    e12, e26 = ema_series(closes, 12), ema_series(closes, 26)
    dif = [x - y for x, y in zip(e12, e26)]
    dea = ema_series(dif, 9)
    hist = [2 * (x - y) for x, y in zip(dif, dea)]
    return {'hist': hist[-1], 'hist_prev': hist[-2], 'golden': dif[-1] > dea[-1]}

def rsi14(closes):
    n = 14
    if len(closes) < n + 1:
        return None
    ag = al = 0.0
    for i in range(1, len(closes)):
        ch = closes[i] - closes[i - 1]
        g, l = max(ch, 0), max(-ch, 0)
        if i == 1:
            ag, al = g, l
        else:
            ag = (g + (n - 1) * ag) / n
            al = (l + (n - 1) * al) / n
    return 100 * ag / (ag + al) if ag + al else 50.0

doc = local_json("/api/boom/data")['data']
json.dump(doc, open('/tmp/boom_doc.json', 'w'), ensure_ascii=False)
stocks = doc['stocks']

# 基本面
codes = [{'code': st['code'], 'isHK': bool(st.get('isHK'))} for st in stocks]
fund = {}
for i in range(0, len(codes), 20):
    r = local_json("/api/stock/batch-prices", {'stocks': codes[i:i + 20]})
    for item in (r.get('data') or []):
        fund[item.get('code', '').upper()] = item
    time.sleep(0.3)

results = []
latest = ""
for idx, st in enumerate(stocks):
    sym = tsymbol(st)
    dk = kline(sym, 'day', 260); time.sleep(0.1)
    wk = kline(sym, 'week', 160); time.sleep(0.1)
    r = {'code': st['code'], 'name': st['name'], 'sectorId': st['sectorId'],
         'role': st.get('role'), 'isHK': bool(st.get('isHK')), 'count': st.get('count') or 0}
    f = fund.get(st['code'].upper(), {})
    for src_k, dst_k in [('price', 'price'), ('pe', 'pe'), ('netprofitYoyRatio', 'yoy'),
                         ('predictNetprofitRatio', 'pred'), ('marketCap', 'marketCap')]:
        r[dst_k] = f.get(src_k) if f.get(src_k) is not None else st.get(src_k)
    if dk:
        closes = [k['close'] for k in dk]; highs = [k['high'] for k in dk]; c = closes[-1]
        r['kdate'] = dk[-1]['date']; latest = max(latest, dk[-1]['date'])
        r['d_close'] = c
        r['d_ma20'] = ma(closes, 20); r['d_ma60'] = ma(closes, 60)
        r['d_high60'] = max(highs[-60:])
        r['d_off_high60'] = (r['d_high60'] - c) / r['d_high60'] * 100
        hi, lo = max(highs[-250:]), min(k['low'] for k in dk[-250:])
        r['d_pos250'] = (c - lo) / (hi - lo) * 100 if hi > lo else 50
        r['d_macd'] = macd_state(closes); r['d_rsi14'] = rsi14(closes[-120:])
        r['closes25'] = closes[-25:]  # 供"破20日线未收回"（连续N日收于MA20下方）判定
        rets = [(closes[i] - closes[i - 1]) / closes[i - 1] for i in range(max(1, len(closes) - 120), len(closes))]
        mean = sum(rets) / len(rets)
        r['ann_vol'] = math.sqrt(sum((x - mean) ** 2 for x in rets) / len(rets)) * math.sqrt(250) * 100
        r['chg20d'] = (c / closes[-21] - 1) * 100 if len(closes) > 21 else None
        r['chg60d'] = (c / closes[-61] - 1) * 100 if len(closes) > 61 else None
    if wk:
        wc = [k['close'] for k in wk]
        r['w_ma20'] = ma(wc, 20); r['w_ma30'] = ma(wc, 30); r['w_macd'] = macd_state(wc)
    if not st.get('isHK') and r.get('pe'):
        c6 = st['code'].split('.')[0]
        fc = c6 + ('01' if st['code'].lower().endswith('.sh') else '02')
        pe_r = curl_json(f"https://emfront.eastmoney.com/APP_HSF10/CPBD/GZFX?code={fc}&year=4&type=1")
        time.sleep(0.1)
        try:
            vals = [float(i['VALUE']) for i in pe_r['data'][0] if i.get('VALUE')]
            if vals:
                r['pe_pct'] = sum(1 for v in vals if v < r['pe']) / len(vals) * 100
        except Exception:
            pass
    results.append(r)
    print(f"{idx + 1}/{len(stocks)} {st['name']} k={len(dk)} w={len(wk)}", file=sys.stderr)

json.dump({'latest_date': latest, 'results': results}, open(OUT, 'w'), ensure_ascii=False)
ok_k = sum(1 for r in results if 'd_close' in r)
ok_pe = sum(1 for r in results if 'pe_pct' in r)
print(f"kline={ok_k}/{len(results)} pe_pct={ok_pe} latest={latest}")
# 停牌检查：K线末日落后于全组最新交易日的标的
for r in results:
    if r.get('kdate') and r['kdate'] < latest:
        print(f"SUSPENDED? {r['name']} last kline {r['kdate']}")
