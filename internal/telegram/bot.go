package telegram

import (
	"bytes"
	"context"
	"errors"
	"github.com/fanonwue/patreon-gobot/internal/db"
	"github.com/fanonwue/patreon-gobot/internal/logging"
	"github.com/fanonwue/patreon-gobot/internal/patreon"
	"github.com/fanonwue/patreon-gobot/internal/tmpl"
	"github.com/fanonwue/patreon-gobot/internal/util"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"gorm.io/gorm"
	"os"
	"slices"
	"strconv"
	"strings"
)

var botInstance *bot.Bot
var botContext context.Context
var privacyPolicyCommand = createPrivacyPolicyCommand()
var htmlEscaper = strings.NewReplacer("<", "&lt;", ">", "&gt;", "&", "&amp;")

var convHandler *ConversationHandler

var telegramCreatorId = 0
var tgPatreonClient = patreon.NewClient(4)

const creatorOnly = true

const (
	stageAddCampaign = iota
)

func StartBot(ctx context.Context) *bot.Bot {
	botContext = ctx
	telegramCreatorId, _ = strconv.Atoi(os.Getenv(util.PrefixEnvVar("TELEGRAM_CREATOR_ID")))

	convEnd := ConversationEnd{
		Command:  "/cancel",
		Function: cancelConversationHandler,
	}

	convHandler = NewConversationHandler(map[int]bot.HandlerFunc{
		stageAddCampaign: noopHandler,
	}, &convEnd)

	opts := []bot.Option{
		bot.WithErrorsHandler(errorHandler),
		bot.WithDefaultHandler(noopHandler),
		bot.WithMiddlewares(middlewares()...),
	}

	botToken := os.Getenv(util.PrefixEnvVar("TELEGRAM_BOT_TOKEN"))
	if botToken == "" {
		panic("No Telegram bot token has been set")
	}

	b, err := bot.New(botToken, opts...)
	if err != nil {
		panic(err)
	}

	commands := commandHandlers()

	registerHandlers(commands, b, botContext)
	registerCommands(commands, b, botContext)

	go func() {
		b.Start(botContext)
	}()
	botInstance = b
	return b
}

func NotifyAvailable(user *db.User, reward *patreon.RewardResult, campaign *patreon.Campaign) {
	buf := new(bytes.Buffer)
	err := rewardAvailableTemplate.Execute(buf, &tmpl.RewardAvailableData{
		Reward:   reward.Reward,
		Campaign: campaign,
	})
	if err != nil {
		logging.Errorf("Error executing template: %v", err)
	}

	botInstance.SendMessage(botContext, &bot.SendMessageParams{
		ChatID:    user.TelegramChatId,
		ParseMode: models.ParseModeHTML,
		Text:      buf.String(),
	})
}

func NotifyMissing(user *db.User, missing []*patreon.RewardResult) {
	if len(missing) == 0 {
		return
	}

	logging.Infof("Notifying about missing rewards: [%s]", util.Join(missing, ", ", func(v *patreon.RewardResult) string {
		return strconv.Itoa(int(v.Id))
	}))
	buf := new(bytes.Buffer)
	err := missingRewardsTemplate.Execute(buf, &tmpl.MissingRewardsData{Rewards: missing})
	if err != nil {
		logging.Errorf("Error executing template: %v", err)
	}

	botInstance.SendMessage(botContext, &bot.SendMessageParams{
		ChatID:    user.TelegramChatId,
		ParseMode: models.ParseModeHTML,
		Text:      buf.String(),
	})
	return
}

// Escape
// Escapes the string (using HTML entities) to make it compatible with Telegram's HTML format.
//
// The API only supports the following entities: `&lt;`, `&gt;` and `&amp;`, therefore only the characters corresponding
// to those will be escaped
func Escape(s string) string {
	return htmlEscaper.Replace(s)
}

func errorHandler(err error) {
	logging.Logf(logging.LevelError, logging.DefaultCalldepth+1, "[TGBOT]: %v", err)
}

func patreonClient() *patreon.Client { return tgPatreonClient }

func middlewares() []bot.Middleware {
	m := []bot.Middleware{
		convHandler.CreateHandlerMiddleware(),
	}

	if creatorOnly && telegramCreatorId > 0 {
		// Prepend creator only middleware to make sure it gets evaluated first
		m = append([]bot.Middleware{creatorOnlyMiddleware}, m...)
	}

	return m
}

func commandHandlers() []*CommandHandler {
	sortedCommands := []*CommandHandler{
		addRewardsCommand(),
		removeRewardsCommand(),
		cancelCommand(),
		listRewardsCommand(),
		resetNotificationsCommand(),
	}

	slices.SortStableFunc(sortedCommands, func(a, b *CommandHandler) int {
		return strings.Compare(a.Pattern, b.Pattern)
	})

	// Add unsorted commands to the bottom
	unsortedCommands := []*CommandHandler{
		startCommand(),
		privacyPolicyCommand,
	}

	commands := append(sortedCommands, unsortedCommands...)
	return commands
}

func noopHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		ParseMode: models.ParseModeHTML,
		Text:      "Not yet implemented",
	})
}

func registerHandlers(commands []*CommandHandler, tgBot *bot.Bot, ctx context.Context) {
	for _, command := range commands {
		handler := command.HandlerFunc
		if command.ChatAction != "" {
			handler = command.ChatActionHandler()
		}

		tgBot.RegisterHandler(command.HandlerType, command.Pattern, command.MatchType, handler)
	}
}

func registerCommands(commands []*CommandHandler, tgBot *bot.Bot, ctx context.Context) {
	tgBot.SetMyCommands(ctx, &bot.SetMyCommandsParams{
		Commands: util.Map(commands, func(ch *CommandHandler) models.BotCommand {
			return models.BotCommand{Command: ch.Pattern, Description: ch.Description}
		}),
	})
}

func cancelConversationHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	// Send a message to indicate the conversation has been cancelled
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "conversation cancelled",
	})
}

func userFromChatId(chatId int64, tx *gorm.DB) (*db.User, bool) {
	if tx == nil {
		tx = db.Db()
	}
	user := &db.User{}
	tx.Limit(1).Find(user, "telegram_chat_id = ?", chatId)
	return user, user.ID > 0
}

func chatIdFromUpdate(update *models.Update) (int64, error) {
	chatId := int64(0)
	if update.Message != nil {
		chatId = update.Message.Chat.ID
	} else if update.CallbackQuery != nil {
		chatId = update.CallbackQuery.Message.Message.Chat.ID
	}

	if chatId == 0 {
		return 0, errors.New("could not determine chat ID")
	}
	return chatId, nil
}

func creatorOnlyMiddleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		// Always allow privacy policy command
		if update.Message != nil && strings.EqualFold(update.Message.Text, privacyPolicyCommand.Pattern) {
			next(ctx, b, update)
			return
		}

		chatId, err := chatIdFromUpdate(update)
		if err != nil {
			return
		}

		if int64(telegramCreatorId) != chatId {
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatId,
				Text:   "This bot is not yet available for the public. If you are interested, please contact this bot's creator (see bot description)",
			})
		} else {
			next(ctx, b, update)
		}
	}
}
