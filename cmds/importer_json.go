// 导入 json 文件

package cmds

import (
	"context"
	"encoding/json"
	"io/ioutil"

	"github.com/axiaoxin-com/investool/models"
	"github.com/axiaoxin-com/logging"
)

// ImportJSON 从 JSON 文件导入数据
func ImportJSON(ctx context.Context, filename string) (models.ExportorDataList, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		logging.Errorf(ctx, "读取文件失败: %v", err)
		return nil, err
	}

	var stocks models.ExportorDataList
	if err := json.Unmarshal(data, &stocks); err != nil {
		logging.Errorf(ctx, "解析 JSON 文件失败: %v", err)
		return nil, err
	}

	logging.Infof(ctx, "成功导入 %d 条股票数据", len(stocks))
	return stocks, nil
}

