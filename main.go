package main

import (
	"context"
	"fmt"
	"github.com/fanonwue/patreon-gobot/internal/db"
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

func main() {
	fmt.Println("Patreon GoBot starting up")
	godotenv.Load()
	db.CreateDatabase()

	appContext, _ := signal.NotifyContext(context.Background(),
		os.Interrupt,
		os.Kill,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)

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

	select {
	case <-appContext.Done():
		fmt.Println("Bot exiting!")
	}
}

func updateInterval() time.Duration {
	interval := 2 * time.Minute
	updateIntervalRaw, err := strconv.Atoi(os.Getenv(util.PrefixEnvVar("UPDATE_INTERVAL")))
	if err == nil {
		interval = time.Duration(updateIntervalRaw) * time.Second

	}
	return interval
}

func StartBackgroundUpdates(ctx context.Context, interval time.Duration) {
	UpdateJob(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			fmt.Println("Stopping BackgroundUpdates")
			// The context is over, stop processing results
			return
		case <-ticker.C:
			UpdateJob(ctx)
		}
	}
}

func UpdateJob(ctx context.Context) {
	fmt.Println("Checking for available rewards")
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
	}), ctx)

	tx := db.Db().Begin()
	var missingRewards []*patreon.RewardResult
	for r := range rewards {
		tr := db.TrackedReward{}
		tx.First(&tr, "user_id = ? AND reward_id = ?", user.ID, r.Id)
		if tx.Error != nil || tr.ID == 0 {
			fmt.Printf("Could not find tracked reward %d for user %d\n", r.Id, user.ID)
			continue
		}

		if r.IsPresent() {
			if r.IsAvailable() {
				onAvailable(user, &r, &tr, c)
			} else {
				tr.AvailableSince = nil
			}

		}

		if r.Status != patreon.RewardFound {
			if !tr.IsMissing {
				tr.IsMissing = true
				missingRewards = append(missingRewards, &r)
			}
			fmt.Printf("Reward not found: %d\n", r.Id)
		}

		tx.Save(&tr)

	}
	telegram.NotifyMissing(user, missingRewards)
	tx.Commit()
}

func onAvailable(user *db.User, r *patreon.RewardResult, tr *db.TrackedReward, client *patreon.Client) {
	fmt.Printf("Reward available: %d\n", r.Id)
	now := time.Now()

	if tr.AvailableSince == nil {
		tr.AvailableSince = &now
	}

	var campaign *patreon.Campaign
	campaignId, _ := r.Reward.CampaignId()
	if campaignId > 0 {
		campaign, _ = client.FetchCampaign(campaignId)
	}

	if campaign == nil {
		r.Status = patreon.RewardErrorNoCampaign
		return
	}

	if tr.LastNotified == nil || tr.AvailableSince.After(*tr.LastNotified) {
		fmt.Printf("Notifying about available reward: %d\n", r.Id)
		telegram.NotifyAvailable(user, r, campaign)
		now := time.Now()
		tr.LastNotified = &now
		if r.Status != patreon.RewardFound {
			tr.IsMissing = true
		}
	}
}
