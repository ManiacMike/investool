package periphera

// 研报 MySQL DAO：List / Get / Upsert(按 dedup_key 去重) / Delete / Import。

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"strings"
	"time"
)

// 列顺序：SELECT / Scan / INSERT 三处必须一致
const researchCols = "id,publish_time,industry_category,institution_name,research_target," +
	"report_type,core_content,target_price,investment_rating,rating_change," +
	"core_catalyst,core_risk_warning,earnings_forecast_adjustment,video_id,author,source_url,created_at"

// newResearchID 生成公开 ID（r_ + 12 hex）
func newResearchID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return "r_" + hex.EncodeToString(b)
}

// dedupKey 去重键：有 video_id 用 video_id，否则用 目标|时间|机构
func dedupKey(r ResearchReport) string {
	if v := strings.TrimSpace(r.VideoID); v != "" {
		return "vid:" + v
	}
	return "k:" + r.ResearchTarget + "|" + r.PublishTime + "|" + r.InstitutionName
}

// ResearchFilter 列表过滤
type ResearchFilter struct {
	Rating      string
	Institution string
	Q           string
}

func scanResearch(rows *sql.Rows) ([]ResearchReport, error) {
	out := []ResearchReport{}
	for rows.Next() {
		var r ResearchReport
		if err := rows.Scan(&r.ID, &r.PublishTime, &r.IndustryCategory, &r.InstitutionName, &r.ResearchTarget,
			&r.ReportType, &r.CoreContent, &r.TargetPrice, &r.InvestmentRating, &r.RatingChange,
			&r.CoreCatalyst, &r.CoreRiskWarning, &r.EarningsForecastAdjustment, &r.VideoID, &r.Author, &r.SourceURL, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListResearch 列表（created_at 倒序，最多 500），支持 rating/institution/q 过滤
func ListResearch(ctx context.Context, f ResearchFilter) ([]ResearchReport, error) {
	db, err := DB()
	if err != nil {
		return nil, err
	}
	q := "SELECT " + researchCols + " FROM research_reports WHERE 1=1"
	var args []interface{}
	if f.Rating != "" && f.Rating != "全部" && f.Rating != "all" {
		q += " AND investment_rating=?"
		args = append(args, f.Rating)
	}
	if f.Institution != "" && f.Institution != "全部机构" {
		q += " AND institution_name=?"
		args = append(args, f.Institution)
	}
	if f.Q != "" {
		q += " AND (research_target LIKE ? OR core_content LIKE ? OR industry_category LIKE ?)"
		like := "%" + f.Q + "%"
		args = append(args, like, like, like)
	}
	q += " ORDER BY created_at DESC LIMIT 500"
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanResearch(rows)
}

// GetResearch 单条；found=false 表示不存在
func GetResearch(ctx context.Context, id string) (ResearchReport, bool, error) {
	db, err := DB()
	if err != nil {
		return ResearchReport{}, false, err
	}
	rows, err := db.QueryContext(ctx, "SELECT "+researchCols+" FROM research_reports WHERE id=?", id)
	if err != nil {
		return ResearchReport{}, false, err
	}
	defer rows.Close()
	list, err := scanResearch(rows)
	if err != nil {
		return ResearchReport{}, false, err
	}
	if len(list) == 0 {
		return ResearchReport{}, false, nil
	}
	return list[0], true, nil
}

// UpsertResearch 按 dedup_key 落库，返回 action: "added" | "updated"
func UpsertResearch(ctx context.Context, r ResearchReport) (string, ResearchReport, error) {
	db, err := DB()
	if err != nil {
		return "", r, err
	}
	now := time.Now().UnixMilli()
	dk := dedupKey(r)

	var existingID string
	err = db.QueryRowContext(ctx, "SELECT id FROM research_reports WHERE dedup_key=?", dk).Scan(&existingID)
	if err == sql.ErrNoRows {
		if r.ID == "" {
			r.ID = newResearchID()
		}
		r.CreatedAt = now
		_, err = db.ExecContext(ctx, "INSERT INTO research_reports ("+researchCols+
			",dedup_key,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
			r.ID, r.PublishTime, r.IndustryCategory, r.InstitutionName, r.ResearchTarget,
			r.ReportType, r.CoreContent, r.TargetPrice, r.InvestmentRating, r.RatingChange,
			r.CoreCatalyst, r.CoreRiskWarning, r.EarningsForecastAdjustment, r.VideoID, r.Author, r.SourceURL, r.CreatedAt,
			dk, now)
		return "added", r, err
	}
	if err != nil {
		return "", r, err
	}
	// 命中已有：按 id 更新（保留原 id / created_at）
	r.ID = existingID
	_, err = db.ExecContext(ctx, "UPDATE research_reports SET publish_time=?,industry_category=?,institution_name=?,"+
		"research_target=?,report_type=?,core_content=?,target_price=?,investment_rating=?,rating_change=?,"+
		"core_catalyst=?,core_risk_warning=?,earnings_forecast_adjustment=?,video_id=?,author=?,source_url=?,dedup_key=?,updated_at=? WHERE id=?",
		r.PublishTime, r.IndustryCategory, r.InstitutionName, r.ResearchTarget, r.ReportType, r.CoreContent,
		r.TargetPrice, r.InvestmentRating, r.RatingChange, r.CoreCatalyst, r.CoreRiskWarning, r.EarningsForecastAdjustment,
		r.VideoID, r.Author, r.SourceURL, dk, now, existingID)
	return "updated", r, err
}

// DeleteResearch 删除
func DeleteResearch(ctx context.Context, id string) (bool, error) {
	db, err := DB()
	if err != nil {
		return false, err
	}
	res, err := db.ExecContext(ctx, "DELETE FROM research_reports WHERE id=?", id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ImportResearch 批量 upsert，返回新增/更新计数
func ImportResearch(ctx context.Context, items []ResearchReport) (added, updated int, err error) {
	for _, it := range items {
		act, _, e := UpsertResearch(ctx, it)
		if e != nil {
			return added, updated, e
		}
		if act == "added" {
			added++
		} else {
			updated++
		}
	}
	return added, updated, nil
}
