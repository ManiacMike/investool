// 基于日 K 线计算常用技术指标：MA MACD RSI
// 算法口径与东方财富/同花顺一致，数据不足的位置返回 NaN

package eastmoney

import (
	"math"
)

// MA 计算 n 日简单移动平均线，返回序列与 K 线一一对应，前 n-1 个位置为 NaN
func (k KLineList) MA(n int) []float64 {
	closes := k.Closes()
	result := make([]float64, len(closes))
	sum := 0.0
	for i, c := range closes {
		sum += c
		if i >= n {
			sum -= closes[i-n]
		}
		if i >= n-1 {
			result[i] = sum / float64(n)
		} else {
			result[i] = math.NaN()
		}
	}
	return result
}

// EMA 计算 n 日指数移动平均，首日以当日收盘价为初始值（国内行情软件口径）
func EMA(values []float64, n int) []float64 {
	result := make([]float64, len(values))
	if len(values) == 0 {
		return result
	}
	alpha := 2.0 / float64(n+1)
	result[0] = values[0]
	for i := 1; i < len(values); i++ {
		result[i] = alpha*values[i] + (1-alpha)*result[i-1]
	}
	return result
}

// MACD 计算 MACD(12,26,9)
// dif = EMA12 - EMA26；dea = dif 的 9 日 EMA；macd 柱 = 2*(dif-dea)（国内行情软件口径）
func (k KLineList) MACD() (dif, dea, macd []float64) {
	closes := k.Closes()
	ema12 := EMA(closes, 12)
	ema26 := EMA(closes, 26)
	dif = make([]float64, len(closes))
	for i := range closes {
		dif[i] = ema12[i] - ema26[i]
	}
	dea = EMA(dif, 9)
	macd = make([]float64, len(closes))
	for i := range closes {
		macd[i] = 2 * (dif[i] - dea[i])
	}
	return
}

// RSI 计算 n 日 RSI（常用 n=14，另有 6、12、24）
// 涨跌幅采用 SMA(X,n,1) 平滑（国内行情软件口径），首日无涨跌返回 NaN
func (k KLineList) RSI(n int) []float64 {
	closes := k.Closes()
	result := make([]float64, len(closes))
	if len(closes) == 0 {
		return result
	}
	result[0] = math.NaN()
	avgGain := 0.0
	avgLoss := 0.0
	for i := 1; i < len(closes); i++ {
		change := closes[i] - closes[i-1]
		gain := math.Max(change, 0)
		loss := math.Max(-change, 0)
		if i == 1 {
			avgGain = gain
			avgLoss = loss
		} else {
			avgGain = (gain + float64(n-1)*avgGain) / float64(n)
			avgLoss = (loss + float64(n-1)*avgLoss) / float64(n)
		}
		if avgGain+avgLoss == 0 {
			result[i] = 50
			continue
		}
		result[i] = 100 * avgGain / (avgGain + avgLoss)
	}
	return result
}
