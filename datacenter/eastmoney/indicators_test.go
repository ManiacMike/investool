package eastmoney

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func testKLineList(closes []float64) KLineList {
	k := KLineList{}
	for _, c := range closes {
		k = append(k, KLine{Close: c})
	}
	return k
}

func TestMA(t *testing.T) {
	k := testKLineList([]float64{1, 2, 3, 4, 5})
	ma := k.MA(3)
	require.True(t, math.IsNaN(ma[0]))
	require.True(t, math.IsNaN(ma[1]))
	require.Equal(t, 2.0, ma[2])
	require.Equal(t, 3.0, ma[3])
	require.Equal(t, 4.0, ma[4])
}

func TestEMA(t *testing.T) {
	ema := EMA([]float64{1, 2, 3}, 3)
	require.Equal(t, 1.0, ema[0])
	require.Equal(t, 1.5, ema[1])
	require.Equal(t, 2.25, ema[2])
}

func TestMACD(t *testing.T) {
	// 价格不变时 dif dea macd 均为 0
	k := testKLineList([]float64{10, 10, 10, 10, 10})
	dif, dea, macd := k.MACD()
	for i := range dif {
		require.Equal(t, 0.0, dif[i])
		require.Equal(t, 0.0, dea[i])
		require.Equal(t, 0.0, macd[i])
	}
}

func TestRSI(t *testing.T) {
	// 连续上涨 RSI 为 100，连续下跌 RSI 为 0
	up := testKLineList([]float64{1, 2, 3, 4, 5}).RSI(14)
	require.True(t, math.IsNaN(up[0]))
	require.Equal(t, 100.0, up[len(up)-1])
	down := testKLineList([]float64{5, 4, 3, 2, 1}).RSI(14)
	require.Equal(t, 0.0, down[len(down)-1])
}
