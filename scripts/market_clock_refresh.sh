#!/bin/zsh
# 每周五收盘后重跑市场风格时钟：launchd 周五触发 + 登录时补跑；上次成功晚于最近一个周五 15:37 则跳过
REPO=/Users/Mike/Workspace/investool
STAMP=$HOME/.market_clock_last_run
LOG=$REPO/scripts/market_clock_refresh.log
export PATH=/usr/local/bin:/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin

now=$(date +%s)
# 最近一个周五 15:37（若今天是周五但未到点，则取上周五）
last_fri=$(date -v-$(( ($(date +%u) + 2) % 7 ))d -v15H -v37M -v0S +%s)
(( last_fri > now )) && last_fri=$(( last_fri - 7*86400 ))
if [[ "$1" != "--force" && -f $STAMP ]] && (( $(cat $STAMP) >= last_fri )); then
  echo "$(date '+%F %T') skip" >> $LOG; exit 0
fi

cd $REPO
echo "$(date '+%F %T') start" >> $LOG
PROMPT='重跑市场风格时钟：用东方财富 kline 接口(push2his.eastmoney.com)拉取上证、深成、创业板、科创50及 AI 链龙头(中际旭创300308、寒武纪688256、新易盛300502、胜宏科技300476、中芯国际688981)和茅台的近 130 日日线，计算 MA20/MA50、距高点幅度、20 日涨跌、两市成交额与涨跌家数；对照 statics/html/market_clock.html 中 PHASES 的六个阶段判断当前位置，重写其中 CURRENT 变量（angle/title/arrow/desc/risk/riskLabel/split/next/watch，desc 以「截至 今天日期」开头、只写数据可验证的事实）以及 watch-sub 文案。最后输出一行不超过 60 字的结论，格式：「阶段：xxx｜风险温度 nn」。'
out=$(claude -p "$PROMPT" --permission-mode bypassPermissions 2>>$LOG)
rc=$?
echo "$out" >> $LOG
if (( rc == 0 )); then
  echo $now > $STAMP
  summary=$(echo "$out" | grep '阶段：' | tail -1)
  osascript -e "display notification \"${summary:-已更新，打开页面查看}\" with title \"市场风格时钟已重跑\" sound name \"Glass\""
  open $REPO/statics/html/market_clock.html
else
  osascript -e 'display notification "重跑失败，查看 scripts/market_clock_refresh.log" with title "市场风格时钟"'
fi
