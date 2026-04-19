package core

import (
	"testing"

	"github.com/axiaoxin-com/investool/models"
	"github.com/axiaoxin-com/logging"
)

func TestCheckFundamentals(t *testing.T) {
	stock := models.Stock{}
	logging.SetLevel("error")
	c := NewChecker(_ctx, DefaultCheckerOptions)
	result, ok := c.CheckFundamentals(_ctx, stock)
	t.Log(ok, result)
}
