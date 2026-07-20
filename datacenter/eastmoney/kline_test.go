package eastmoney

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetSecID(t *testing.T) {
	require.Equal(t, "1.600149", _em.GetSecID("600149.sh"))
	require.Equal(t, "0.000001", _em.GetSecID("000001.SZ"))
	require.Equal(t, "0.835185", _em.GetSecID("835185.BJ"))
	require.Equal(t, "", _em.GetSecID("600149"))
}

func TestQueryDailyKLine(t *testing.T) {
	d, err := _em.QueryDailyKLine(_ctx, "600149.sh", 250)
	require.Nil(t, err)
	require.NotEmpty(t, d)
	t.Log(d[len(d)-1])
}
