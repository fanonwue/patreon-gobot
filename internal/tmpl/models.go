package tmpl

import (
	"cmp"
	"github.com/fanonwue/patreon-gobot/internal/patreon"
	"slices"
)

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

func compareByAmount(first, last *patreon.Reward) int {
	result := cmp.Compare(first.Attributes.AmountCents, last.Attributes.AmountCents)
	if result == 0 {
		result = cmp.Compare(first.Title(), last.Title())
	}
	return result
}

func (lc *ListCampaign) RewardsSortedByAmountAscending() []*patreon.Reward {
	return slices.SortedFunc(slices.Values(lc.Rewards), func(a, b *patreon.Reward) int {
		return compareByAmount(a, b)
	})
}

func (lc *ListCampaign) RewardsSortedByAmountDescending() []*patreon.Reward {
	return slices.SortedFunc(slices.Values(lc.Rewards), func(a, b *patreon.Reward) int {
		return compareByAmount(b, a)
	})
}
