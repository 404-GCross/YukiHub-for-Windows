package enums

type GameStatus string

// 游玩状态与手机版 YukiHub 完全一致，只有五态，没有「想玩」。
//
// 手机版 GameRepository.normalizePlayStatus 只会产出 unplayed / playing /
// completed / onhold / dropped 五个值，桌面端从上游带来的 not_started 与
// want_to_play 已按 docs/mobile-yukihub-migration.md 合并到 unplayed。
const (
	StatusUnplayed  GameStatus = "unplayed"  // 未玩
	StatusPlaying   GameStatus = "playing"   // 游玩中
	StatusCompleted GameStatus = "completed" // 已通关
	StatusOnHold    GameStatus = "onhold"    // 搁置
	StatusDropped   GameStatus = "dropped"   // 抛弃
)

var AllGameStatuses = []struct {
	Value  GameStatus
	TSName string
}{
	{StatusUnplayed, "UNPLAYED"},
	{StatusPlaying, "PLAYING"},
	{StatusCompleted, "COMPLETED"},
	{StatusOnHold, "ON_HOLD"},
	{StatusDropped, "DROPPED"},
}
