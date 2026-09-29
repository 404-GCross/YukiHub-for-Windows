package enums

type SourceType string

const (
	Local      SourceType = "local"
	Bangumi    SourceType = "bangumi"
	VNDB       SourceType = "vndb"
	Ymgal      SourceType = "ymgal"
	Steam      SourceType = "steam"
	DLsite     SourceType = "dlsite"
	TouchGal   SourceType = "touchgal"
	Hikarinagi SourceType = "hikarinagi"

	ErogameScape SourceType = "erogamescape"

	// BangumiMirror 与 Bangumi 共用同一套 API 与 token，仅站点域名不同。
	BangumiMirror SourceType = "bangumi_mirror"
	// NextMoe 未萌目录，使用用户级 OAuth 令牌访问 catalog 接口。
	NextMoe SourceType = "nextmoe"
)

var AllSourceTypes = []struct {
	Value  SourceType
	TSName string
}{
	{Local, "LOCAL"},
	{Bangumi, "BANGUMI"},
	{VNDB, "VNDB"},
	{Ymgal, "YMGAL"},
	{Steam, "STEAM"},
	{DLsite, "DLSITE"},
	{TouchGal, "TOUCHGAL"},
	{Hikarinagi, "HIKARINAGI"},
	{ErogameScape, "EROGAMESCAPE"},
	{BangumiMirror, "BANGUMI_MIRROR"},
	{NextMoe, "NEXTMOE"},
}
