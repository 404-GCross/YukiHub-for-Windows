package enums

type GameListSortBy string

const (
	GameListSortByName         GameListSortBy = "name"
	GameListSortByCompany      GameListSortBy = "company"
	GameListSortByLastPlayedAt GameListSortBy = "last_played_at"
	GameListSortByCreatedAt    GameListSortBy = "created_at"
	GameListSortByRating       GameListSortBy = "rating"
	GameListSortByReleaseDate  GameListSortBy = "release_date"
	GameListSortByPlayTime     GameListSortBy = "play_time"
)

var AllGameListSortByTypes = []struct {
	Value  GameListSortBy
	TSName string
}{
	{GameListSortByName, "NAME"},
	{GameListSortByCompany, "COMPANY"},
	{GameListSortByLastPlayedAt, "LAST_PLAYED_AT"},
	{GameListSortByCreatedAt, "CREATED_AT"},
	{GameListSortByRating, "RATING"},
	{GameListSortByReleaseDate, "RELEASE_DATE"},
	{GameListSortByPlayTime, "PLAY_TIME"},
}

type SortOrder string

const (
	SortOrderAsc  SortOrder = "asc"
	SortOrderDesc SortOrder = "desc"
)

var AllSortOrderTypes = []struct {
	Value  SortOrder
	TSName string
}{
	{SortOrderAsc, "ASC"},
	{SortOrderDesc, "DESC"},
}
