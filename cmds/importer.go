// 导入各类型的数据结果

package cmds

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/axiaoxin-com/investool/models"
	"github.com/axiaoxin-com/logging"
)

// Importer 导入器实例
type Importer struct {
	Stocks models.ExportorDataList
}

// Import 导入数据
func Import(ctx context.Context, importFilename string) error {
	beginTime := time.Now()
	fileext := strings.ToLower(path.Ext(importFilename))

	logging.Infof(ctx, "investool importer start import from %s", importFilename)

	var err error
	var stocks models.ExportorDataList

	switch fileext {
	case ".json":
		stocks, err = ImportJSON(ctx, importFilename)
	default:
		return fmt.Errorf("不支持的文件格式: %s，目前仅支持 .json 格式", fileext)
	}

	if err != nil {
		return err
	}

	importer := Importer{
		Stocks: stocks,
	}

	// 显示导入结果
	importer.Display(ctx)

	fmt.Printf(
		"\ninvestool importer import %s success, total:%d latency:%#vs\n",
		fileext,
		len(stocks),
		time.Now().Sub(beginTime).Seconds(),
	)

	return nil
}

// Display 显示导入的数据
func (i Importer) Display(ctx context.Context) {
	if len(i.Stocks) == 0 {
		logging.Warnf(ctx, "导入的数据为空")
		return
	}

	fmt.Printf("\n导入的股票数据摘要:\n")
	fmt.Printf("总计: %d 只股票\n\n", len(i.Stocks))

	// 显示前10条数据
	displayCount := 10
	if len(i.Stocks) < displayCount {
		displayCount = len(i.Stocks)
	}

	fmt.Printf("前 %d 条数据:\n", displayCount)
	fmt.Printf("%-10s %-20s %-15s %-10s %-10s\n", "代码", "名称", "行业", "价格", "ROE")
	fmt.Printf("%s\n", strings.Repeat("-", 75))

	for idx := 0; idx < displayCount; idx++ {
		stock := i.Stocks[idx]
		fmt.Printf("%-10s %-20s %-15s %-10.2f %-10.2f\n",
			stock.Code,
			stock.Name,
			stock.Industry,
			stock.Price,
			stock.LatestROE,
		)
	}

	if len(i.Stocks) > displayCount {
		fmt.Printf("\n... 还有 %d 条数据未显示\n", len(i.Stocks)-displayCount)
	}

	// 显示行业统计
	industryList := i.Stocks.GetIndustryList()
	if len(industryList) > 0 {
		fmt.Printf("\n行业分布 (共 %d 个行业):\n", len(industryList))
		for _, industry := range industryList {
			count := 0
			for _, stock := range i.Stocks {
				if stock.Industry == industry {
					count++
				}
			}
			fmt.Printf("  %s: %d 只\n", industry, count)
		}
	}
}

