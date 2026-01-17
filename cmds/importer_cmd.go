// 导入器 cli command

package cmds

import (
	"context"
	"fmt"

	"github.com/axiaoxin-com/logging"
	"github.com/urfave/cli/v2"
)

const (
	// ProcessorImporter 导入器
	ProcessorImporter = "importer"
)

var (
	// DefaultImportFilename 要导入的文件名默认值
	DefaultImportFilename = "./dist/investool.json"
)

// FlagsImporter importer cli flags
func FlagsImporter() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:        "filename",
			Aliases:     []string{"f"},
			Value:       DefaultImportFilename,
			Usage:       `指定导入文件名`,
			EnvVars:     []string{"XSTOCK_IMPORTER_FILENAME"},
			DefaultText: DefaultImportFilename,
		},
	}
}

// ActionImporter cli action
func ActionImporter() func(c *cli.Context) error {
	return func(c *cli.Context) error {
		ctx := context.Background()
		loglevel := c.String("loglevel")
		logging.SetLevel(loglevel)

		filename := c.String("filename")
		if filename == "" {
			return fmt.Errorf("请指定要导入的文件名")
		}

		logging.Debugf(ctx, "importer params: filename=%s", filename)
		if err := Import(ctx, filename); err != nil {
			return err
		}
		return nil
	}
}

// CommandImporter 导入器 cli command
func CommandImporter() *cli.Command {
	flags := FlagsImporter()
	cmd := &cli.Command{
		Name:      ProcessorImporter,
		Usage:     "股票数据导入器",
		UsageText: "从 JSON 文件导入股票数据。支持的文件格式：[json]",
		Flags:     flags,
		Action:    ActionImporter(),
	}
	return cmd
}

