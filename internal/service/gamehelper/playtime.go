package gamehelper

import (
	"context"
	"database/sql"
	"fmt"

	"yukihub/internal/utils"
)

// playTimeJoinClause 把「有效游玩时长」以 1:1 的方式挂到游戏行上，供排序使用。
//
// 与 QueryGamesPlayTime 共用同一套清零语义，避免「按时长排序却排到已被清零的旧时长」。
// 子查询按 game_id 分组，每组至多一行，因此 LEFT JOIN 不会放大结果集行数。
const playTimeJoinClause = `
	LEFT JOIN (
		SELECT
			ps.game_id,
			COALESCE(SUM(COALESCE(ps.duration, 0)), 0) AS total_play_time
		FROM play_sessions ps
		LEFT JOIN games g ON g.id = ps.game_id
		WHERE g.playtime_reset_at IS NULL
		   OR COALESCE(ps.end_time, ps.start_time) >= g.playtime_reset_at
		GROUP BY ps.game_id
	) playtime ON playtime.game_id = g.id
`

// PlayTimeJoinClause 返回按有效游玩时长排序所需的 JOIN 片段。
func PlayTimeJoinClause() string {
	return playTimeJoinClause
}

// PlayTimeOrderTerm 生成按有效游玩时长排序的 ORDER BY 片段。
//
// 时长为 0 或缺失（没有有效会话）的游戏恒排在末尾，不受升降序影响，
// 与 last_played_at 的排序处理保持一致。direction 取 "ASC" / "DESC"。
func PlayTimeOrderTerm(direction string) string {
	return fmt.Sprintf(
		"COALESCE(playtime.total_play_time, 0) = 0 ASC, COALESCE(playtime.total_play_time, 0) %s",
		direction,
	)
}

// QueryGamesPlayTime 按游戏批量统计有效游玩时长，返回 game_id → 秒 的映射。
//
// 语义与导出方向（exporter/yukihub.go 的 buildYukiHubGame）保持一致：
//   - 桌面端库内 play_sessions.duration 的单位是秒，这里原样返回秒，
//     不在这一层做毫秒换算（导出时才 ×1000）；
//   - 清零（games.playtime_reset_at）之前的会话不计入统计，记录本身保留。
//     判定对齐 Android 侧 exportPlaySessionsJson：
//     COALESCE(end_time, start_time) >= playtime_reset_at。
//
// gameIDs 为空表示不限游戏，返回库内全部有会话的时长；
// 没有任何有效会话的游戏不会出现在结果里，调用方按缺省 0 处理。
func QueryGamesPlayTime(ctx context.Context, db *sql.DB, gameIDs []string) (map[string]int64, error) {
	result := make(map[string]int64)
	if db == nil {
		return result, fmt.Errorf("database is not initialized")
	}

	args := make([]interface{}, 0, len(gameIDs))
	whereSQL := ""
	if len(gameIDs) > 0 {
		whereSQL = fmt.Sprintf("AND ps.game_id IN (%s)", utils.BuildPlaceholders(len(gameIDs)))
		for _, id := range gameIDs {
			args = append(args, id)
		}
	}

	query := fmt.Sprintf(`
		SELECT ps.game_id, COALESCE(SUM(COALESCE(ps.duration, 0)), 0)
		FROM play_sessions ps
		LEFT JOIN games g ON g.id = ps.game_id
		-- 括号不能省：AND 的优先级高于 OR，否则追加的 id 过滤只会作用在清零判定上。
		WHERE (g.playtime_reset_at IS NULL
		   OR COALESCE(ps.end_time, ps.start_time) >= g.playtime_reset_at)
		%s
		GROUP BY ps.game_id
	`, whereSQL)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query games play time: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var gameID string
		var playTime int64
		if err := rows.Scan(&gameID, &playTime); err != nil {
			return nil, fmt.Errorf("scan game play time: %w", err)
		}
		result[gameID] = playTime
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate game play time rows: %w", err)
	}
	return result, nil
}
