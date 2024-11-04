package main

import (
	"context"
	"github.com/fanonwue/patreon-gobot/internal/db"
	"github.com/fanonwue/patreon-gobot/internal/logging"
	"github.com/fanonwue/patreon-gobot/internal/patreon"
	"github.com/fanonwue/patreon-gobot/internal/telegram"
	"github.com/fanonwue/patreon-gobot/internal/util"
	"github.com/joho/godotenv"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

const minimumUpdateInterval = 30 * time.Second

func main() {
	appContext, _ := setup()
	_ = telegram.StartBot(appContext)

	//user := db.User{}
	//user.ID = 1
	//user.TelegramChatId = 465753188
	//db.Db().Create(&user)
	//
	//rewardIds := []patreon.RewardId{7996793, 6521499, 9049320, 21951894, 8101799, 7996804, 4066319, 10206990, 10207065}
	//for _, rewardId := range rewardIds {
	//	db.Db().Create(&db.TrackedReward{
	//		UserID:   user.ID,
	//		RewardId: int64(rewardId),
	//	})
	//}

	go StartBackgroundUpdates(appContext, updateInterval())

	<-appContext.Done()
	logging.Info("Bot exiting!")
}

func setup() (context.Context, context.CancelFunc) {
	dotenvErr := godotenv.Load()
	logLevelErr := logging.SetLogLevelFromEnvironment(util.PrefixEnvVar("LOG_LEVEL"))
	if dotenvErr != nil {
		logging.Debugf("error loading .env file: %v", dotenvErr)
	}
	if logLevelErr != nil {
		logging.Errorf("error setting log level: %v", logLevelErr)
	}

	logging.Info("---- BOT STARTING ----")
	logging.Info("Welcome to Patreon GoBot!")
	db.CreateDatabase()

	appContext, cancel := signal.NotifyContext(context.Background(),
		os.Interrupt,
		os.Kill,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)

	patreon.OnStartup(appContext)

	return appContext, cancel
}

func updateInterval() time.Duration {
	interval := 2 * time.Minute
	updateIntervalRaw, err := strconv.Atoi(os.Getenv(util.PrefixEnvVar("UPDATE_INTERVAL")))
	if err == nil {
		interval = time.Duration(updateIntervalRaw) * time.Second
	}
	if interval < minimumUpdateInterval {
		logging.Warnf("UPDATE_INTERVAL set too low, setting it to the minimum interval of %.0f seconds", minimumUpdateInterval.Seconds())
		interval = minimumUpdateInterval
	}
	return interval
}

func StartBackgroundUpdates(ctx context.Context, interval time.Duration) {
	UpdateJob(ctx)
	logging.Infof("Starting background updates at an interval of %.0f seconds", interval.Seconds())
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			logging.Info("Stopping BackgroundUpdates")
			// The context is over, stop processing results
			return
		case <-ticker.C:
			UpdateJob(ctx)
		}
	}
}

func UpdateJob(ctx context.Context) {
	logging.Debug("Checking for available rewards")
	users := make([]db.User, 0)
	db.Db().Find(&users)

	wg := sync.WaitGroup{}
	// Do checks synchronously for now to prevent any massive rate limiting
	for _, user := range users {
		wg.Add(1)
		go updateForUser(&user, ctx, func() {
			wg.Done()
		})
	}

	wg.Wait()
}

func updateForUser(user *db.User, ctx context.Context, doneCallback func()) {
	defer doneCallback()
	db.Db().Preload("Rewards").First(&user)
	c := patreon.NewClient(4)
	rewards := c.FetchRewardsSlice(util.Map(user.Rewards, func(tr db.TrackedReward) patreon.RewardId {
		return patreon.RewardId(tr.RewardId)
	}), true, ctx)

	tx := db.Db().Begin()
	var missingRewards []*patreon.RewardResult
	for r := range rewards {
		tr := db.TrackedReward{}
		tx.First(&tr, "user_id = ? AND reward_id = ?", user.ID, r.Id)
		if tx.Error != nil || tr.ID == 0 {
			logging.Warnf("Could not find tracked reward %d for user %d", r.Id, user.ID)
			continue
		}

		if r.Status == patreon.RewardErrorRateLimit {
			logging.Warnf("Got rate limited for reward: %d", r.Id)
			continue
		}

		if r.Status != patreon.RewardFound {
			if !tr.IsMissing {
				tr.IsMissing = true
				missingRewards = append(missingRewards, &r)
				// Don't repeat the warning if the reward is known to be missing already
				logging.Warnf("Reward not found: %d", r.Id)
			}
		} else {
			tr.IsMissing = false
		}

		if r.IsPresent() {
			if r.IsAvailable() {
				onAvailable(user, &r, &tr, c)
			} else {
				tr.AvailableSince = nil
			}
		}

		tx.Save(&tr)

	}
	telegram.NotifyMissing(user, missingRewards)
	tx.Commit()
}

func onAvailable(user *db.User, r *patreon.RewardResult, tr *db.TrackedReward, client *patreon.Client) {
	logging.Debugf("Reward available: %d", r.Id)
	now := time.Now()

	if tr.AvailableSince == nil {
		tr.AvailableSince = &now
	}

	var campaign *patreon.Campaign
	campaignId, _ := r.Reward.CampaignId()
	if campaignId > 0 {
		campaign, _ = client.FetchCampaign(campaignId, false)
	}

	if campaign == nil {
		r.Status = patreon.RewardErrorNoCampaign
		return
	}

	if tr.LastNotified == nil || tr.AvailableSince.After(*tr.LastNotified) {
		telegram.NotifyAvailable(user, r, campaign)
		now := time.Now()
		tr.LastNotified = &now
		if r.Status != patreon.RewardFound {
			tr.IsMissing = true
		}
	}
}
