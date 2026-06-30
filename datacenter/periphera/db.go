package periphera

// 研报 MySQL 连接。凭据走环境变量，绝不写入被 git 跟踪的 config.toml。
//   PERIPHERA_MYSQL_DSN       完整 DSN（最高优先）
//   或分项：PERIPHERA_MYSQL_{USER,PASSWORD,HOST,PORT,DB}
//   默认：root:<空>@tcp(127.0.0.1:3306)/periphera
// 本机：export PERIPHERA_MYSQL_PASSWORD=*** （见 PERIPHERA_TODO.md）

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var (
	dbConn *sql.DB
	dbOnce sync.Once
	dbErr  error
)

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func mysqlDSN() string {
	if d := os.Getenv("PERIPHERA_MYSQL_DSN"); d != "" {
		return d
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true&loc=Local",
		envOr("PERIPHERA_MYSQL_USER", "root"),
		os.Getenv("PERIPHERA_MYSQL_PASSWORD"),
		envOr("PERIPHERA_MYSQL_HOST", "127.0.0.1"),
		envOr("PERIPHERA_MYSQL_PORT", "3306"),
		envOr("PERIPHERA_MYSQL_DB", "periphera"),
	)
}

// DB 返回惰性初始化的数据库连接；连接失败时返回 err，调用方据此降级。
func DB() (*sql.DB, error) {
	dbOnce.Do(func() {
		d, err := sql.Open("mysql", mysqlDSN())
		if err != nil {
			dbErr = err
			return
		}
		d.SetMaxOpenConns(10)
		d.SetMaxIdleConns(5)
		d.SetConnMaxLifetime(time.Hour)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := d.PingContext(ctx); err != nil {
			dbErr = fmt.Errorf("periphera mysql ping: %w", err)
			return
		}
		dbConn = d
	})
	return dbConn, dbErr
}
