package periphera

// 研报采集管道:后端起 node 子进程(scripts/douyin_to_report.mjs --stream)驱动已登录
// Chrome(CDP)枚举抖音博主主页视频、逐条走豆包分析,脚本按 NDJSON 逐行吐事件到 stdout。
// 本包逐行读取:遇到 video_done 事件即用现有 ImportResearch 落 MySQL,并把 added/updated
// 回填进事件后转发给上层(HTTP 流式响应)。持久化归属在 Go 侧,脚本流式模式不自连后台。
//
// 单 Chrome / 单豆包会话不可并行,用包级互斥保证同一时刻仅一个采集任务。
//
// 可用环境变量覆盖运行参数:
//   PERIPHERA_DOUYIN_NODE     node 解释器路径(默认 node)
//   PERIPHERA_DOUYIN_SCRIPT   采集脚本路径(默认 scripts/douyin_to_report.mjs)
//   CDP_PORT                  Chrome 远程调试端口(透传给脚本,默认脚本内 9222)

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// collectMu 保证同一时刻仅一个采集任务在跑(单 Chrome/单豆包会话)。
var (
	collectMu  sync.Mutex
	collecting bool
)

// TryStartCollect 抢占采集锁;已有任务在跑时返回 false。
func TryStartCollect() bool {
	collectMu.Lock()
	defer collectMu.Unlock()
	if collecting {
		return false
	}
	collecting = true
	return true
}

// FinishCollect 释放采集锁(与 TryStartCollect 成对,建议 defer 调用)。
func FinishCollect() {
	collectMu.Lock()
	collecting = false
	collectMu.Unlock()
}

// collectEvent 仅解出转发/落库需要的字段;其余字段原样透传。
type collectEvent struct {
	Type    string           `json:"type"`
	Records []ResearchReport `json:"records"`
}

// RunCollect 启动采集子进程并把每行事件通过 emit 转发。
//   - input:  抖音博主主页链接或单视频链接
//   - limit:  主页模式最多采集条数
//   - prompt: 页面可编辑的豆包分析 prompt(其中 {{video_id}} 为动态占位符);为空时脚本用内置模板
//   - emit:   每行事件回调(已是完整一行 JSON 的字节,不含换行);上层负责 flush
//
// video_done 事件在转发前先落库(ImportResearch),并回填 added/updated。
// 调用方需自行持有采集锁(TryStartCollect / FinishCollect)。
func RunCollect(ctx context.Context, input string, limit int, prompt string, emit func([]byte)) error {
	node := envOr("PERIPHERA_DOUYIN_NODE", "node")
	script := envOr("PERIPHERA_DOUYIN_SCRIPT", "scripts/douyin_to_report.mjs")

	// 预检:DB 连不上就别启动采集(否则每条视频白跑一遍豆包最后存不进去)。
	if _, dberr := DB(); dberr != nil {
		emit(mustJSON(map[string]any{"type": "error",
			"error": "数据库未连接,采集中止(研报无法落库):" + dberr.Error() +
				" —— 请设置 PERIPHERA_MYSQL_PASSWORD/DSN 后重启服务"}))
		return dberr
	}

	// 采集前取库中已有 video_id,交给脚本在豆包分析前跳过(去重前置,省时省钱)。
	skipIDs := existingVideoIDs(ctx)

	args := []string{script, "--profile", input, "--stream", "--limit", strconv.Itoa(limit)}
	if skipIDs != "" {
		args = append(args, "--skip-ids", skipIDs)
	}

	cmd := exec.CommandContext(ctx, node, args...)
	// 页面传入的 prompt 走环境变量注入(多行/含引号更安全,且不进 argv);为空则脚本回退内置模板。
	if p := strings.TrimSpace(prompt); p != "" {
		cmd.Env = append(os.Environ(), "PERIPHERA_DOUYIN_PROMPT="+prompt)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		emit(mustJSON(map[string]any{"type": "error", "error": "无法创建子进程管道: " + err.Error()}))
		return err
	}
	// stderr 收集用于诊断(脚本人类日志走 stderr)。
	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		emit(mustJSON(map[string]any{"type": "error", "error": "启动采集脚本失败: " + err.Error()}))
		return err
	}

	sawTerminal := false // 是否已见 done/error 终止事件
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024) // 单条 records 可能较大,放宽行上限
	for scanner.Scan() {
		line := scanner.Bytes()
		trimmed := strings.TrimSpace(string(line))
		if trimmed == "" {
			continue
		}
		var ev collectEvent
		if json.Unmarshal([]byte(trimmed), &ev) != nil {
			// 非法行(理论上不会发生):原样透传,避免吞掉信息。
			emit([]byte(trimmed))
			continue
		}
		switch ev.Type {
		case "done", "error":
			sawTerminal = true
		}
		if ev.Type == "video_done" && len(ev.Records) > 0 {
			// 落库并回填 added/updated 后转发。
			added, updated, dberr := ImportResearch(ctx, ev.Records)
			out := decodeLine(trimmed)
			if dberr != nil {
				out["db_error"] = dberr.Error()
			} else {
				out["added"] = added
				out["updated"] = updated
			}
			emit(mustJSON(out))
			continue
		}
		emit([]byte(trimmed))
	}

	waitErr := cmd.Wait()
	// 脚本异常退出且未吐终止事件时,补发一个 error,保证前端一定收到收尾。
	if !sawTerminal {
		msg := "采集进程异常结束"
		if waitErr != nil {
			msg += ": " + waitErr.Error()
		}
		if s := strings.TrimSpace(stderr.String()); s != "" {
			// 附最后几行 stderr 便于定位。
			msg += " | " + lastLines(s, 3)
		}
		emit(mustJSON(map[string]any{"type": "error", "error": msg}))
	}
	return waitErr
}

// existingVideoIDs 取库中所有非空 video_id,逗号拼接;库不可用时返回空串(不影响采集,仅失去去重)。
func existingVideoIDs(ctx context.Context) string {
	all, err := ListResearch(ctx, ResearchFilter{})
	if err != nil {
		return ""
	}
	ids := make([]string, 0, len(all))
	for _, r := range all {
		if v := strings.TrimSpace(r.VideoID); v != "" {
			ids = append(ids, v)
		}
	}
	return strings.Join(ids, ",")
}

// decodeLine 把一行事件 JSON 解成 map 以便回填字段;失败则返回仅含 type 的兜底。
func decodeLine(line string) map[string]any {
	var m map[string]any
	if json.Unmarshal([]byte(line), &m) == nil {
		return m
	}
	return map[string]any{"type": "video_done"}
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte(`{"type":"error","error":"internal marshal error"}`)
	}
	return b
}

// lastLines 取字符串末尾 n 行,拼成单行(便于塞进错误消息)。
func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, " ⏎ ")
}
