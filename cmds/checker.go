// 对给定股票名/股票代码进行检测

package cmds

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/axiaoxin-com/investool/core"
	"github.com/axiaoxin-com/investool/models"
	"github.com/axiaoxin-com/logging"
	"github.com/olekukonko/tablewriter"
)

// Check 对给定名称或代码进行检测，输出检测结果
func Check(ctx context.Context, keywords []string, opts core.CheckerOptions) (results map[string]core.CheckResult, err error) {
	results = make(map[string]core.CheckResult)
	searcher := core.NewSearcher(ctx)
	stocks, err := searcher.SearchStocks(ctx, keywords)
	if err != nil {
		logging.Fatal(ctx, err.Error())
	}

	for _, stock := range stocks {
		checker := core.NewChecker(ctx, opts)
		checkResult, ok := checker.CheckFundamentals(ctx, stock)
		k := fmt.Sprintf("%s-%s", stock.BaseInfo.SecurityNameAbbr, stock.BaseInfo.Secucode)
		results[k] = checkResult

		if opts.OutputFormat == "markdown" {
			if !ok {
				renderMarkdown(checkResult, []string{k, "FAILED"}, stock)
			} else {
				renderMarkdown(checkResult, []string{k, "OK"}, stock)
			}
		} else {
			// 默认使用表格输出
			table := newTable()
			if !ok {
				renderTable(table, checkResult, []string{k, "FAILED"}, stock)
			} else {
				renderTable(table, checkResult, []string{k, "OK"}, stock)
			}
		}
	}
	return results, nil
}

func newTable() *tablewriter.Table {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetRowLine(true)
	headers := []string{"检测指标", "检测结果"}
	table.SetHeader(headers)
	table.SetHeaderColor(
		tablewriter.Colors{tablewriter.Bold, tablewriter.BgBlackColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.BgBlackColor},
	)
	return table
}

func renderTable(table *tablewriter.Table, checkResult core.CheckResult, footers []string, stock models.Stock) {
	footerValColor := tablewriter.FgRedColor
	if footers[1] == "OK" {
		footerValColor = tablewriter.FgGreenColor
	}
	table.SetFooter(footers)
	table.SetFooterColor(
		tablewriter.Colors{tablewriter.Bold, footerValColor},
		tablewriter.Colors{tablewriter.Bold, footerValColor},
	)
	for k, m := range checkResult {
		row := []string{k, strings.ReplaceAll(m["desc"], "<br/>", "\n")}

		if m["ok"] == "false" {
			table.Rich(
				row,
				[]tablewriter.Colors{{tablewriter.Bold, tablewriter.BgRedColor}, {tablewriter.Bold, tablewriter.BgRedColor}},
			)
		} else {
			table.Append(row)
		}
	}

	// 添加巴菲特评分
	buffettRow := []string{"巴菲特评分", fmt.Sprintf("总分: %.1f分\n%s",
		stock.BuffettScore.TotalScore,
		strings.ReplaceAll(stock.BuffettScore.ScoreDescription, "<br/>", "\n"))}
	table.Append(buffettRow)

	table.Render()
}

// renderMarkdown 以Markdown格式输出检测结果
func renderMarkdown(checkResult core.CheckResult, footers []string, stock models.Stock) {
	// 输出标题
	fmt.Printf("## %s 检测结果: %s\n\n", footers[0], footers[1])

	// 输出表格头部
	fmt.Println("| 检测指标 | 检测结果 |")
	fmt.Println("| --- | --- |")

	// 输出表格内容
	for k, m := range checkResult {
		desc := strings.ReplaceAll(m["desc"], "<br/>", "<br>")
		if m["ok"] == "false" {
			// 失败项目使用高亮标记
			fmt.Printf("| **%s** | **%s** |\n", k, desc)
		} else {
			fmt.Printf("| %s | %s |\n", k, desc)
		}
	}

	// 添加巴菲特评分
	buffettDesc := strings.ReplaceAll(stock.BuffettScore.ScoreDescription, "\n", "<br>")
	fmt.Printf("| 巴菲特评分 | 总分: %.1f分<br>%s |\n",
		stock.BuffettScore.TotalScore,
		buffettDesc)

	// 输出结果
	if footers[1] == "OK" {
		fmt.Printf("\n**检测结果: %s** ✅\n\n", footers[1])
	} else {
		fmt.Printf("\n**检测结果: %s** ❌\n\n", footers[1])
	}
}
