// 获取日 K 线

package eastmoney

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/axiaoxin-com/goutils"
	"github.com/axiaoxin-com/logging"
	"go.uber.org/zap"
)

// RespKLine 日 K 线接口返回结构
type RespKLine struct {
	Rc   int `json:"rc"`
	Data struct {
		Code   string   `json:"code"`
		Market int      `json:"market"`
		Name   string   `json:"name"`
		Klines []string `json:"klines"`
	} `json:"data"`
}

// KLine 一根日 K 线
type KLine struct {
	// 日期，格式 2022-01-28
	Date string
	// 开盘价
	Open float64
	// 收盘价
	Close float64
	// 最高价
	High float64
	// 最低价
	Low float64
	// 成交量（手）
	Volume float64
	// 成交额（元）
	Amount float64
	// 振幅（%）
	Amplitude float64
	// 涨跌幅（%）
	ChangePercent float64
	// 涨跌额
	Change float64
	// 换手率（%）
	TurnoverRate float64
}

// KLineList 日 K 线列表，按日期从旧到新排列
type KLineList []KLine

// Closes 返回收盘价序列
func (k KLineList) Closes() []float64 {
	closes := make([]float64, len(k))
	for i, item := range k {
		closes[i] = item.Close
	}
	return closes
}

// GetSecID 生成 secid 请求参数
func (e EastMoney) GetSecID(secuCode string) string {
	secuCode = strings.ToUpper(secuCode)
	if strings.HasSuffix(secuCode, ".SH") {
		return "1." + strings.TrimSuffix(secuCode, ".SH")
	}
	if strings.HasSuffix(secuCode, ".SZ") {
		return "0." + strings.TrimSuffix(secuCode, ".SZ")
	}
	if strings.HasSuffix(secuCode, ".BJ") {
		return "0." + strings.TrimSuffix(secuCode, ".BJ")
	}
	return ""
}

// QueryDailyKLine 获取前复权日 K 线，limit 为返回的 K 线数量
func (e EastMoney) QueryDailyKLine(ctx context.Context, secuCode string, limit int) (KLineList, error) {
	secid := e.GetSecID(secuCode)
	if secid == "" {
		return nil, fmt.Errorf("QueryDailyKLine invalid secuCode:%s", secuCode)
	}
	apiurl := "https://push2his.eastmoney.com/api/qt/stock/kline/get"
	params := map[string]string{
		"secid":   secid,
		"fields1": "f1,f2,f3,f4,f5,f6",
		"fields2": "f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61",
		"klt":     "101", // 日线
		"fqt":     "1",   // 前复权
		"beg":     "0",
		"end":     "20500101",
		"lmt":     strconv.Itoa(limit),
	}
	logging.Debug(ctx, "EastMoney QueryDailyKLine "+apiurl+" begin", zap.Any("params", params))
	beginTime := time.Now()
	apiurl, err := goutils.NewHTTPGetURLWithQueryString(ctx, apiurl, params)
	if err != nil {
		return nil, err
	}
	resp := RespKLine{}
	err = goutils.HTTPGET(ctx, e.HTTPClient, apiurl, nil, &resp)
	latency := time.Now().Sub(beginTime).Milliseconds()
	logging.Debug(
		ctx,
		"EastMoney QueryDailyKLine "+apiurl+" end",
		zap.Int64("latency(ms)", latency),
	)
	if err != nil {
		return nil, err
	}
	result := KLineList{}
	for _, kline := range resp.Data.Klines {
		fields := strings.Split(kline, ",")
		if len(fields) != 11 {
			logging.Error(ctx, "QueryDailyKLine invalid kline:"+kline)
			continue
		}
		values := make([]float64, 10)
		parseErr := false
		for i, field := range fields[1:] {
			value, err := strconv.ParseFloat(field, 64)
			if err != nil {
				logging.Error(ctx, "QueryDailyKLine ParseFloat error:"+err.Error())
				parseErr = true
				break
			}
			values[i] = value
		}
		if parseErr {
			continue
		}
		result = append(result, KLine{
			Date:          fields[0],
			Open:          values[0],
			Close:         values[1],
			High:          values[2],
			Low:           values[3],
			Volume:        values[4],
			Amount:        values[5],
			Amplitude:     values[6],
			ChangePercent: values[7],
			Change:        values[8],
			TurnoverRate:  values[9],
		})
	}
	return result, nil
}
