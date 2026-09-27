package test

import (
	"yukihub/internal/common/enums"
	"yukihub/internal/common/vo"
	"yukihub/internal/models"
	"yukihub/internal/service"
)

func addGameViaMetadata(gameService *service.GameService, game models.Game) error {
	source := game.SourceType
	if source == "" {
		source = enums.Local
	}

	return gameService.AddGameFromWebMetadata(vo.GameMetadataFromWebVO{
		Source: source,
		Game:   game,
	})
}
