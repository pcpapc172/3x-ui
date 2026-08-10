package resellerbot

import (
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func updateBotUsername(ownerId int, username string) {
	if username == "" {
		return
	}
	database.GetDB().Model(&model.BotConfig{}).Where("owner_id = ?", ownerId).Update("bot_username", username)
}

func configFor(ownerId int) (*model.BotConfig, error) {
	cfg := &model.BotConfig{}
	err := database.GetDB().Where("owner_id = ?", ownerId).First(cfg).Error
	return cfg, err
}

func isOwnerChat(ownerId int, chatID int64) bool {
	cfg, err := configFor(ownerId)
	if err != nil || cfg.OwnerChatId == 0 {
		return false
	}
	return cfg.OwnerChatId == chatID
}
