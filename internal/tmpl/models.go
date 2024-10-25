package tmpl

import "github.com/fanonwue/patreon-gobot/internal/patreon"

type (
	ListCampaign struct {
		Campaign *patreon.Campaign
		Rewards  []*patreon.Reward
	}

	ListTemplateData struct {
		Campaigns []*ListCampaign
	}

	MissingRewardsData struct {
		Rewards []*patreon.RewardResult
	}

	RewardAvailableData struct {
		Reward   *patreon.Reward
		Campaign *patreon.Campaign
	}
)

func (lc *ListCampaign) AddReward(reward *patreon.Reward) {
	lc.Rewards = append(lc.Rewards, reward)
}
