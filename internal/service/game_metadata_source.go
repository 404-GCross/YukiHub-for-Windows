package service

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"yukihub/internal/common/enums"
	"yukihub/internal/models"
	"yukihub/internal/models/yukihub"
	"yukihub/internal/service/cloudsync"
	"yukihub/internal/service/gamehelper"
	"yukihub/internal/utils/dbutils"
	"yukihub/internal/utils/metadata"
)

func scanGameMetadataSources(rows *sql.Rows) ([]models.GameMetadataSource, error) {
	items := make([]models.GameMetadataSource, 0)
	for rows.Next() {
		var item models.GameMetadataSource
		var sourceType string
		if err := rows.Scan(
			&item.GameID,
			&sourceType,
			&item.SourceID,
			&item.CacheJSON,
			&item.CachedAt,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("读取游戏元数据来源失败: %w", err)
		}
		item.SourceType = enums.SourceType(sourceType)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历游戏元数据来源失败: %w", err)
	}
	return items, nil
}

func (s *GameService) GetGameMetadataSources(gameID string) ([]models.GameMetadataSource, error) {
	rows, err := s.db.QueryContext(s.ctx, `
		SELECT game_id, source_type, source_id, COALESCE(cache_json, ''),
		       COALESCE(cached_at, created_at, updated_at, CURRENT_TIMESTAMP),
		       COALESCE(created_at, CURRENT_TIMESTAMP),
		       COALESCE(updated_at, created_at, CURRENT_TIMESTAMP)
		FROM game_metadata_sources
		WHERE game_id = ?
		ORDER BY source_type
	`, strings.TrimSpace(gameID))
	if err != nil {
		return nil, fmt.Errorf("查询游戏元数据来源失败: %w", err)
	}
	defer rows.Close()
	items, err := scanGameMetadataSources(rows)
	if err != nil || len(items) > 0 {
		return items, err
	}

	var legacySource string
	var legacyID string
	var cachedAt time.Time
	var createdAt time.Time
	var updatedAt time.Time
	err = s.db.QueryRowContext(s.ctx, `
		SELECT COALESCE(source_type, ''), COALESCE(source_id, ''),
		       COALESCE(cached_at, created_at, updated_at, CURRENT_TIMESTAMP),
		       COALESCE(created_at, CURRENT_TIMESTAMP),
		       COALESCE(updated_at, cached_at, created_at, CURRENT_TIMESTAMP)
		FROM games WHERE id = ?
	`, strings.TrimSpace(gameID)).Scan(&legacySource, &legacyID, &cachedAt, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return items, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取兼容元数据来源失败: %w", err)
	}
	source := gamehelper.NormalizeMetadataSourceType(enums.SourceType(legacySource))
	if source == "" || source == enums.Local || strings.TrimSpace(legacyID) == "" {
		return items, nil
	}
	return []models.GameMetadataSource{{
		GameID: gameID, SourceType: source, SourceID: strings.TrimSpace(legacyID),
		CachedAt: cachedAt, CreatedAt: createdAt, UpdatedAt: updatedAt,
	}}, nil
}

func (s *GameService) getGameMetadataSource(gameID string, source enums.SourceType) (models.GameMetadataSource, error) {
	source = gamehelper.NormalizeMetadataSourceType(source)
	var item models.GameMetadataSource
	var sourceType string
	err := s.db.QueryRowContext(s.ctx, `
		SELECT game_id, source_type, source_id, COALESCE(cache_json, ''),
		       COALESCE(cached_at, created_at, updated_at, CURRENT_TIMESTAMP),
		       COALESCE(created_at, CURRENT_TIMESTAMP),
		       COALESCE(updated_at, created_at, CURRENT_TIMESTAMP)
		FROM game_metadata_sources
		WHERE game_id = ? AND source_type = ?
	`, gameID, string(source)).Scan(
		&item.GameID,
		&sourceType,
		&item.SourceID,
		&item.CacheJSON,
		&item.CachedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return models.GameMetadataSource{}, fmt.Errorf("游戏未关联元数据来源 %s", source)
	}
	if err != nil {
		return models.GameMetadataSource{}, fmt.Errorf("查询游戏元数据来源失败: %w", err)
	}
	item.SourceType = enums.SourceType(sourceType)
	return item, nil
}

func (s *GameService) UpsertGameMetadataSource(gameID string, source enums.SourceType, sourceID string) error {
	source, sourceID, err := gamehelper.NormalizeMetadataSource(source, sourceID)
	if err != nil {
		return err
	}
	gameID = strings.TrimSpace(gameID)
	if gameID == "" {
		return fmt.Errorf("游戏 ID 不能为空")
	}
	return dbutils.WithDuckDBWriteLock(s.db, func() error {
		return dbutils.RetryDuckDBWriteConflict(s.ctx, func() error {
			return s.upsertGameMetadataSourceRecord(gameID, source, sourceID)
		})
	})
}

func (s *GameService) upsertGameMetadataSourceRecord(gameID string, source enums.SourceType, sourceID string) error {
	tx, err := s.db.BeginTx(s.ctx, nil)
	if err != nil {
		return fmt.Errorf("开始保存元数据来源事务失败: %w", err)
	}
	defer tx.Rollback()
	if err := s.upsertGameMetadataSourceTx(tx, gameID, source, sourceID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *GameService) upsertGameMetadataSourceTx(tx *sql.Tx, gameID string, source enums.SourceType, sourceID string) error {
	var exists bool
	if err := tx.QueryRowContext(s.ctx, `SELECT EXISTS(SELECT 1 FROM games WHERE id = ?)`, gameID).Scan(&exists); err != nil {
		return fmt.Errorf("检查游戏记录失败: %w", err)
	}
	if !exists {
		return fmt.Errorf("game not found with id: %s", gameID)
	}

	now := time.Now()
	if _, err := tx.ExecContext(s.ctx, `
		INSERT INTO game_metadata_sources (
			game_id, source_type, source_id, cached_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (game_id, source_type) DO UPDATE SET
			source_id = EXCLUDED.source_id,
			cached_at = EXCLUDED.cached_at,
			updated_at = EXCLUDED.updated_at
	`, gameID, string(source), sourceID, now, now, now); err != nil {
		return fmt.Errorf("保存游戏元数据来源失败: %w", err)
	}

	if _, err := tx.ExecContext(s.ctx, `
		UPDATE games
		SET source_type = CASE
				WHEN LOWER(TRIM(COALESCE(source_type, ''))) IN ('', 'local') THEN ?
				ELSE source_type
			END,
			source_id = CASE
				WHEN LOWER(TRIM(COALESCE(source_type, ''))) IN ('', 'local')
				  OR LOWER(TRIM(COALESCE(source_type, ''))) = ? THEN ?
				ELSE source_id
			END,
			updated_at = ?
		WHERE id = ?
	`, string(source), string(source), sourceID, now, gameID); err != nil {
		return fmt.Errorf("更新游戏默认元数据来源失败: %w", err)
	}

	if err := cloudsync.DeleteTombstone(s.ctx, tx, cloudsync.EntityGameMetadataSource, cloudsync.MetadataSourceTombstoneID(gameID, string(source))); err != nil {
		return err
	}
	return nil
}

func (s *GameService) SetDefaultMetadataSource(gameID string, source enums.SourceType) error {
	return dbutils.WithDuckDBWriteLock(s.db, func() error {
		return dbutils.RetryDuckDBWriteConflict(s.ctx, func() error {
			return s.setDefaultMetadataSourceRecord(gameID, source)
		})
	})
}

func (s *GameService) setDefaultMetadataSourceRecord(gameID string, source enums.SourceType) error {
	tx, err := s.db.BeginTx(s.ctx, nil)
	if err != nil {
		return fmt.Errorf("开始设置默认元数据来源事务失败: %w", err)
	}
	defer tx.Rollback()
	if err := s.setDefaultMetadataSourceTx(tx, gameID, source); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *GameService) setDefaultMetadataSourceTx(tx *sql.Tx, gameID string, source enums.SourceType) error {
	gameID = strings.TrimSpace(gameID)
	source = gamehelper.NormalizeMetadataSourceType(source)

	var sourceID string
	err := tx.QueryRowContext(s.ctx, `
		SELECT source_id
		FROM game_metadata_sources
		WHERE game_id = ? AND source_type = ?
	`, gameID, string(source)).Scan(&sourceID)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("游戏未关联元数据来源 %s", source)
	}
	if err != nil {
		return fmt.Errorf("查询游戏元数据来源失败: %w", err)
	}

	result, err := tx.ExecContext(s.ctx, `
		UPDATE games
		SET source_type = ?, source_id = ?, updated_at = ?
		WHERE id = ?
	`, string(source), sourceID, time.Now(), gameID)
	if err != nil {
		return fmt.Errorf("设置默认元数据来源失败: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("game not found with id: %s", gameID)
	}
	return nil
}

func (s *GameService) DeleteGameMetadataSource(gameID string, source enums.SourceType) error {
	gameID = strings.TrimSpace(gameID)
	source = gamehelper.NormalizeMetadataSourceType(source)
	return dbutils.WithDuckDBWriteLock(s.db, func() error {
		return dbutils.RetryDuckDBWriteConflict(s.ctx, func() error {
			return s.deleteGameMetadataSourceRecord(gameID, source)
		})
	})
}

func (s *GameService) deleteGameMetadataSourceRecord(gameID string, source enums.SourceType) error {
	tx, err := s.db.BeginTx(s.ctx, nil)
	if err != nil {
		return fmt.Errorf("开始删除元数据来源事务失败: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(s.ctx, `
		DELETE FROM game_metadata_sources WHERE game_id = ? AND source_type = ?
	`, gameID, string(source))
	if err != nil {
		return fmt.Errorf("删除游戏元数据来源失败: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("游戏未关联元数据来源 %s", source)
	}

	var defaultSource string
	if err := tx.QueryRowContext(s.ctx, `SELECT COALESCE(source_type, '') FROM games WHERE id = ?`, gameID).Scan(&defaultSource); err != nil {
		return fmt.Errorf("读取默认元数据来源失败: %w", err)
	}
	if gamehelper.NormalizeMetadataSourceType(enums.SourceType(defaultSource)) == source {
		nextSource, nextID, selectErr := s.selectNextDefaultMetadataSource(tx, gameID)
		if selectErr != nil {
			return selectErr
		}
		if _, err := tx.ExecContext(s.ctx, `
			UPDATE games
			SET source_type = ?, source_id = ?, updated_at = ?
			WHERE id = ?
		`, gamehelper.DefaultMetadataSourceValue(nextSource), nextID, time.Now(), gameID); err != nil {
			return fmt.Errorf("更新默认元数据来源失败: %w", err)
		}
	}

	if err := cloudsync.UpsertTombstone(s.ctx, tx, cloudsync.EntityGameMetadataSource, cloudsync.MetadataSourceTombstoneID(gameID, string(source)), time.Now()); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *GameService) selectNextDefaultMetadataSource(tx *sql.Tx, gameID string) (enums.SourceType, string, error) {
	rows, err := tx.QueryContext(s.ctx, `SELECT source_type, source_id FROM game_metadata_sources WHERE game_id = ?`, gameID)
	if err != nil {
		return "", "", fmt.Errorf("查询备选元数据来源失败: %w", err)
	}
	defer rows.Close()
	available := make(map[enums.SourceType]string)
	for rows.Next() {
		var source string
		var sourceID string
		if err := rows.Scan(&source, &sourceID); err != nil {
			return "", "", fmt.Errorf("读取备选元数据来源失败: %w", err)
		}
		available[enums.SourceType(source)] = sourceID
	}
	for _, source := range s.getConfiguredMetadataSources() {
		if sourceID, ok := available[source]; ok {
			return source, sourceID, nil
		}
	}
	for source, sourceID := range available {
		return source, sourceID, nil
	}
	return "", "", nil
}

func (s *GameService) addInitialMetadataSourcesTx(tx *sql.Tx, game models.Game) error {
	sources := game.MetadataSources
	if len(sources) == 0 && game.SourceType != "" && game.SourceType != enums.Local && strings.TrimSpace(game.SourceID) != "" {
		sources = []models.GameMetadataSource{{SourceType: game.SourceType, SourceID: game.SourceID}}
	}
	for _, source := range sources {
		normalizedSource, normalizedSourceID, err := gamehelper.NormalizeMetadataSource(source.SourceType, source.SourceID)
		if err != nil {
			return err
		}
		if err := s.upsertGameMetadataSourceTx(tx, game.ID, normalizedSource, normalizedSourceID); err != nil {
			return err
		}
	}
	defaultSource := game.SourceType
	if defaultSource == enums.Local {
		defaultSource = ""
	}
	if defaultSource != "" {
		if err := s.setDefaultMetadataSourceTx(tx, game.ID, defaultSource); err != nil {
			return err
		}
	}
	return nil
}

// updateMetadataSourceCachePayload 写入某个来源的元数据负载缓存。
//
// 只 UPDATE 已有来源行，不新建来源：来源身份由 UpsertGameMetadataSource 负责，
// 这里只负责缓存内容，避免缓存写入意外改变了「游戏关联了哪些来源」。
func (s *GameService) updateMetadataSourceCachePayload(gameID string, source enums.SourceType, payload string) error {
	if strings.TrimSpace(payload) == "" {
		return nil
	}
	return dbutils.WithDuckDBWriteLock(s.db, func() error {
		return dbutils.RetryDuckDBWriteConflict(s.ctx, func() error {
			now := time.Now()
			if _, err := s.db.ExecContext(s.ctx, `
				UPDATE game_metadata_sources
				SET cache_json = ?, updated_at = ?
				WHERE game_id = ? AND source_type = ?
			`, payload, now, gameID, string(source)); err != nil {
				return fmt.Errorf("保存元数据缓存失败: %w", err)
			}
			return nil
		})
	})
}

// saveScrapedMetadataCache 把一次刮削结果写成该来源的元数据缓存负载。
//
// 缓存键是 (游戏, 来源)，两者缺一就跳过：没有来源身份的缓存无法在导出到 Android
// 时被关联，写进去只是噪声。缓存写失败属于非致命问题，由调用方决定是否告警。
func (s *GameService) saveScrapedMetadataCache(game models.Game, result metadata.MetadataResult) error {
	source := gamehelper.NormalizeMetadataSourceType(result.Game.SourceType)
	if source == "" || source == enums.Local {
		source = gamehelper.NormalizeMetadataSourceType(game.SourceType)
	}
	if source == "" || source == enums.Local {
		return nil
	}
	sourceID := metadataSourceIDFor(game, source)
	if sourceID == "" {
		return nil
	}
	payload, err := encodeMetadataCachePayload(sourceID, result)
	if err != nil {
		return err
	}
	return s.updateMetadataSourceCachePayload(game.ID, source, payload)
}

// metadataSourceIDFor 取某个来源在该游戏上的标识，优先用逐来源列表，其次回退默认来源。
func metadataSourceIDFor(game models.Game, source enums.SourceType) string {
	for _, item := range game.MetadataSources {
		if gamehelper.NormalizeMetadataSourceType(item.SourceType) == source {
			return strings.TrimSpace(item.SourceID)
		}
	}
	if gamehelper.NormalizeMetadataSourceType(game.SourceType) == source {
		return strings.TrimSpace(game.SourceID)
	}
	return ""
}

// encodeMetadataCachePayload 把一个刮削结果编码为 Android 版 VnMetadata JSON。
//
// 只填两端同名的字段；桌面端没有对应概念的字段（截图、封面分级、罗马音标题）留空，
// 不做猜测性填充，避免污染对端展示。JSON 结构对齐 docs/mobile-yukihub-migration.md。
func encodeMetadataCachePayload(sourceID string, result metadata.MetadataResult) (string, error) {
	payload := yukihub.Metadata{
		ID:            strings.TrimSpace(sourceID),
		ChineseTitle:  strings.TrimSpace(result.Game.Name),
		OriginalTitle: firstNonEmptyString(result.Game.Aliases...),
		CoverURL:      strings.TrimSpace(result.Game.CoverURL),
		Description:   strings.TrimSpace(result.Game.Summary),
		Released:      strings.TrimSpace(result.Game.ReleaseDate),
		Developer:     strings.TrimSpace(result.Game.Company),
		TagsText:      strings.Join(metadataTagNames(result.Tags), ","),
		RatingText:    formatMetadataRating(result.Game.Rating),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("序列化元数据缓存失败: %w", err)
	}
	return string(data), nil
}

func metadataTagNames(tags []metadata.TagItem) []string {
	names := make([]string, 0, len(tags))
	for _, tag := range tags {
		if name := strings.TrimSpace(tag.Name); name != "" {
			names = append(names, name)
		}
	}
	return names
}

// formatMetadataRating 与 Android 侧的 ratingText 对齐：只写数值，无评分时留空。
func formatMetadataRating(rating float64) string {
	if rating <= 0 {
		return ""
	}
	return strconv.FormatFloat(rating, 'f', -1, 64)
}
