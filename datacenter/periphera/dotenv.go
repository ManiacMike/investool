package periphera

// 轻量 .env 加载：服务启动时把项目根 .env 注入进程环境（仅当该变量尚未设置，
// 即真实环境变量优先）。让 SEED_API_KEY / PERIPHERA_MYSQL_* 等无需手动 export。
// 不引第三方依赖；.env 已 gitignore，凭据不入库。

import (
	"bufio"
	"os"
	"strings"
	"sync"
)

var dotenvOnce sync.Once

// LoadDotEnv 幂等加载项目根 .env（找不到则静默跳过）。
func LoadDotEnv() {
	dotenvOnce.Do(func() {
		// 服务从仓库根运行用 .env；测试 cwd 在包目录，向上多找几级
		for _, path := range []string{".env", "../.env", "../../.env", "../../../.env"} {
			if loadEnvFile(path) {
				return
			}
		}
	})
}

func loadEnvFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		k, v, _ := strings.Cut(line, "=")
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		v = strings.Trim(v, `"'`)
		if k == "" {
			continue
		}
		if _, exists := os.LookupEnv(k); !exists {
			os.Setenv(k, v)
		}
	}
	return true
}
