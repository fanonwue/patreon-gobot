package telegram

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"github.com/fanonwue/patreon-gobot/internal/db"
	"github.com/fanonwue/patreon-gobot/internal/logging"
	"github.com/fanonwue/patreon-gobot/internal/patreon"
	"github.com/fanonwue/patreon-gobot/internal/tmpl"
	"github.com/fanonwue/patreon-gobot/internal/util"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"gorm.io/gorm"
	"maps"
	"slices"
	"strconv"
	"strings"
)

func parseIdList(message string) []int {
	var ids []int
	splitByComma := strings.Split(message, ",")
	for _, segment := range splitByComma {
		splitBySpace := strings.Split(segment, " ")
		for _, rawId := range splitBySpace {
			id := strings.TrimSpace(rawId)
			if id == "" {
				continue
			}
			parsedId, err := strconv.Atoi(id)
			if err != nil {
				continue
			}
			ids = append(ids, parsedId)
		}
	}
	return ids
}

func addRewardsCommand() *CommandHandler {
	return &CommandHandler{
		Pattern:     "/add",
		Description: "Adds one or more Rewards IDs to the list of observed rewards",
		HandlerType: bot.HandlerTypeMessageText,
		MatchType:   bot.MatchTypePrefix,
		HandlerFunc: addRewardsHandler,
		ChatAction:  models.ChatActionTyping,
	}
}

func addRewardsHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatId := update.Message.Chat.ID
	reply := models.ReplyParameters{
		MessageID: update.Message.ID,
	}

	ids := parseIdList(update.Message.Text)

	if len(ids) == 0 {
		sendMessage(ctx, &bot.SendMessageParams{
			ChatID:          chatId,
			ReplyParameters: &reply,
			Text:            "No valid reward IDs provided",
		})
		return
	}

	user, _ := userFromChatId(chatId, nil)
	db.Db().Preload("Rewards").Find(user)
	existingRewardIds := util.Map(user.Rewards, func(r db.TrackedReward) int {
		return int(r.RewardId)
	})

	var newRewardIds []patreon.RewardId
	for _, id := range ids {
		if !slices.Contains(existingRewardIds, id) {
			newRewardIds = append(newRewardIds, patreon.RewardId(id))
		}
	}

	if len(newRewardIds) == 0 {
		sendMessage(ctx, &bot.SendMessageParams{
			ChatID:          chatId,
			ReplyParameters: &reply,
			Text:            "No new reward ID found",
		})
		return
	}

	var foundIds []patreon.RewardId

	for r := range patreonClient().FetchRewardsSlice(newRewardIds, ctx) {
		if r.IsPresent() {
			foundIds = append(foundIds, r.Id)
		}
	}

	var savedRewards []string
	tx := db.Db().Begin()
	for _, id := range foundIds {
		tracked := db.TrackedReward{RewardId: int64(id), UserID: user.ID}
		tx.Save(&tracked)
		if tracked.ID > 0 {
			savedRewards = append(savedRewards, strconv.Itoa(int(tracked.RewardId)))
		}
	}
	tx.Commit()
	savedRewardsJoined := strings.Join(savedRewards, ", ")
	sendMessage(ctx, &bot.SendMessageParams{
		ChatID:          chatId,
		ReplyParameters: &reply,
		Text:            fmt.Sprintf("Now tracking rewards [%s]", savedRewardsJoined),
	})
	logging.Infof("Added rewards [%s] for user %d (Chat ID: %d)", savedRewardsJoined, user.ID, user.TelegramChatId)
}

func removeRewardsCommand() *CommandHandler {
	return &CommandHandler{
		Pattern:     "/remove",
		Description: "Remove one or more Rewards IDs from the list of observed rewards",
		HandlerType: bot.HandlerTypeMessageText,
		MatchType:   bot.MatchTypePrefix,
		HandlerFunc: removeRewardsHandler,
		ChatAction:  models.ChatActionTyping,
	}
}

func removeRewardsHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatId := update.Message.Chat.ID
	reply := models.ReplyParameters{
		MessageID: update.Message.ID,
	}

	ids := parseIdList(update.Message.Text)

	if len(ids) == 0 {
		sendMessage(ctx, &bot.SendMessageParams{
			ChatID:          chatId,
			ReplyParameters: &reply,
			Text:            "No valid reward IDs provided",
		})
		return
	}

	user, _ := userFromChatId(chatId, nil)

	var removedRewards []string
	tx := db.Db().Begin()
	for _, id := range ids {
		tx.Unscoped().Delete(&db.TrackedReward{}, "user_id = ? AND reward_id = ?", user.ID, id)
		if tx.Error == nil {
			removedRewards = append(removedRewards, strconv.Itoa(id))
		}
	}
	tx.Commit()
	removedRewardsJoined := strings.Join(removedRewards, ", ")
	sendMessage(ctx, &bot.SendMessageParams{
		ChatID:          chatId,
		ReplyParameters: &reply,
		Text:            fmt.Sprintf("Removed rewards [%s]", removedRewardsJoined),
	})
	logging.Infof("Removed rewards [%s] for user %d (Chat ID: %d)", removedRewardsJoined, user.ID, user.TelegramChatId)
}

func listRewardsCommand() *CommandHandler {
	return &CommandHandler{
		Pattern:     "/list",
		Description: "Shows a list of currently tracked rewards",
		HandlerType: bot.HandlerTypeMessageText,
		MatchType:   bot.MatchTypeExact,
		HandlerFunc: listRewardsHandler,
		ChatAction:  models.ChatActionTyping,
	}
}

func listRewardsHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatId := update.Message.Chat.ID
	user, _ := userFromChatId(chatId, nil)
	db.Db().Preload("Rewards").Find(user)

	campaigns := map[patreon.CampaignId]*tmpl.ListCampaign{}

	rewardResults := patreonClient().FetchRewardsSlice(util.Map(user.Rewards, func(r db.TrackedReward) patreon.RewardId {
		return patreon.RewardId(r.RewardId)
	}), ctx)

	var missingRewards []*patreon.RewardResult

	for result := range rewardResults {
		if !result.IsPresent() {
			missingRewards = append(missingRewards, &result)
			continue
		}

		r := result.Reward

		campaignId, err := r.CampaignId()
		if err != nil {
			result.Status = patreon.RewardErrorNoCampaign
			missingRewards = append(missingRewards, &result)
			continue
		}

		listCampaign, found := campaigns[campaignId]
		if !found {
			campaign, err := patreonClient().FetchCampaign(campaignId)
			if err != nil {
				result.Status = patreon.RewardErrorNoCampaign
				missingRewards = append(missingRewards, &result)
				continue
			}
			listCampaign = &tmpl.ListCampaign{Campaign: campaign, Rewards: []*patreon.Reward{}}
			campaigns[campaignId] = listCampaign
		}

		listCampaign.AddReward(r)
	}

	listCampaigns := slices.SortedFunc(maps.Values(campaigns), func(a, b *tmpl.ListCampaign) int {
		return cmp.Compare(a.Campaign.Name(), b.Campaign.Name())
	})

	buf := new(bytes.Buffer)
	err := listRewardsTemplate.Execute(buf, &tmpl.ListTemplateData{Campaigns: listCampaigns})
	if err != nil {
		logging.Errorf("Error executing template: %v", err)
	}

	disableLinkPreview := true

	sendMessage(ctx, &bot.SendMessageParams{
		ChatID:             chatId,
		LinkPreviewOptions: &models.LinkPreviewOptions{IsDisabled: &disableLinkPreview},
		ParseMode:          models.ParseModeHTML,
		Text:               buf.String(),
	})

	if len(missingRewards) == 0 {
		return
	}

	buf.Reset()
	err = missingRewardsTemplate.Execute(buf, &tmpl.MissingRewardsData{Rewards: missingRewards})
	if err != nil {
		logging.Errorf("Error executing template: %v", err)
	}

	sendMessage(ctx, &bot.SendMessageParams{
		ChatID:             chatId,
		LinkPreviewOptions: &models.LinkPreviewOptions{IsDisabled: &disableLinkPreview},
		ParseMode:          models.ParseModeHTML,
		Text:               buf.String(),
	})
}

func resetNotificationsCommand() *CommandHandler {
	return &CommandHandler{
		Pattern:     "/reset_notifications",
		Description: "Resets the notification tracker. Previous notifications for (still) available rewards will be sent again.",
		HandlerType: bot.HandlerTypeMessageText,
		MatchType:   bot.MatchTypeExact,
		HandlerFunc: resetNotificationsHandler,
		ChatAction:  models.ChatActionTyping,
	}
}

func resetNotificationsHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatId := update.Message.Chat.ID
	reply := models.ReplyParameters{
		MessageID: update.Message.ID,
	}

	tx := db.Db().Begin()
	user, _ := userFromChatId(chatId, tx)
	tx.Model(&db.TrackedReward{}).Where("user_id = ?", user.ID).Update("last_notified", gorm.Expr("NULL"))
	if tx.Error != nil {
		tx.Rollback()
		logging.Errorf("Error resetting rewards: %v", tx.Error)
		sendMessage(ctx, &bot.SendMessageParams{
			ChatID:          chatId,
			ReplyParameters: &reply,
			Text:            "Error resetting rewards",
		})
		return
	}
	tx.Commit()
	sendMessage(ctx, &bot.SendMessageParams{
		ChatID:          chatId,
		ReplyParameters: &reply,
		Text:            "Notifications reset",
	})
	logging.Infof("Notifications reset for user %d (Chat ID: %d)", user.ID, user.TelegramChatId)
}

func cancelCommand() *CommandHandler {
	return &CommandHandler{
		Pattern:     "/cancel",
		Description: "Cancels any active conversation",
		HandlerType: bot.HandlerTypeMessageText,
		MatchType:   bot.MatchTypeExact,
		HandlerFunc: cancelConversationHandler,
		ChatAction:  models.ChatActionTyping,
	}
}

func startCommand() *CommandHandler {
	return &CommandHandler{
		Pattern:     "/start",
		Description: "Starts bot interaction",
		HandlerType: bot.HandlerTypeMessageText,
		MatchType:   bot.MatchTypeExact,
		HandlerFunc: startHandler,
		ChatAction:  models.ChatActionTyping,
	}
}

func startHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatId := update.Message.Chat.ID
	tx := db.Db().Begin()
	user, userFound := userFromChatId(chatId, tx)

	if userFound {
		sendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatId,
			Text:   "You are already registered. Welcome back!",
		})
		tx.Commit()
		return
	}

	user.TelegramChatId = chatId
	tx.Create(&user)
	tx.Commit()

	sendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatId,
		ParseMode: models.ParseModeHTML,
		Text:      "You have been registered as a user. You can start adding rewards that you'd like to track via tha /add command.",
	})
	logging.Infof("Registered new user %d (Chat ID: %d)", user.ID, user.TelegramChatId)
}

func createPrivacyPolicyCommand() *CommandHandler {
	return &CommandHandler{
		Pattern:     "/privacy",
		Description: "Privacy policy",
		HandlerType: bot.HandlerTypeMessageText,
		MatchType:   bot.MatchTypeExact,
		HandlerFunc: privacyPolicyHandler,
		ChatAction:  models.ChatActionTyping,
	}
}

func privacyPolicyHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	sendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		ParseMode: models.ParseModeHTML,
		Text:      fmt.Sprintf(privacyPolicyTemplate, update.Message.Chat.ID),
	})
}
