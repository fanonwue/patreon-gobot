package patreon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/fanonwue/patreon-gobot/internal/logging"
	"iter"
	"net/http"
	"net/url"
	"slices"
	"sync"
)

type RewardStatus int

const (
	RewardFound RewardStatus = iota
	RewardErrorUnknown
	RewardErrorForbidden
	RewardErrorNotFound
	RewardErrorNoCampaign
	RewardErrorRateLimit
)

type (
	ResponseCodeError struct {
		StatusCode int
		Message    string
	}

	Client struct {
		MaxParallelism int
		httpClient     *http.Client
	}

	RewardResult struct {
		Id     RewardId
		Reward *Reward
		Status RewardStatus
	}
)

func (r *RewardResult) IsPresent() bool {
	return r.Reward != nil
}

func (r *RewardResult) IsAvailable() bool {
	return r.IsPresent() && r.Reward.IsAvailable()
}

func (r *ResponseCodeError) Error() string {
	return fmt.Sprintf("received status %d: %s", r.StatusCode, r.Message)
}

func NewClient(maxParallelism int) *Client {
	return &Client{
		MaxParallelism: maxParallelism,
		httpClient:     &http.Client{},
	}
}

func (c *Client) CheckAvailability(rewardIds []RewardId, ctx context.Context) <-chan RewardResult {
	rewardResults := make(chan RewardResult)

	go func() {
		defer close(rewardResults)
		for rewardResult := range c.FetchRewardsSlice(rewardIds, ctx) {
			if rewardResult.Status != RewardFound {
				rewardResults <- rewardResult
				continue
			}

			if rewardResult.IsAvailable() {
				rewardResults <- rewardResult
				continue
			}
		}
	}()

	return rewardResults
}

func (c *Client) fetchRewardInternal(id RewardId, rewardChannel chan<- RewardResult, callback func()) {
	defer callback()
	putInChannel := false
	ra := RewardResult{
		Id: id,
	}
	defer func() {
		if putInChannel {
			rewardChannel <- ra
		}
	}()
	logging.Debugf("Fetching reward %d", id)
	reward, err := c.FetchReward(id)

	if err == nil {
		ra.Reward = reward
		putInChannel = true
	} else {
		var responseCodeError *ResponseCodeError
		if errors.As(err, &responseCodeError) {
			putInChannel = true
			ra.Status = RewardErrorUnknown

			switch responseCodeError.StatusCode {
			case http.StatusForbidden:
				ra.Status = RewardErrorForbidden
			case http.StatusNotFound:
				ra.Status = RewardErrorNotFound
			case http.StatusTooManyRequests:
				ra.Status = RewardErrorRateLimit
			}
		} else {
			logging.Errorf("Error fetching reward %d: %v", id, err)
		}
	}
}

func (c *Client) FetchRewardsSlice(rewardIds []RewardId, ctx context.Context) <-chan RewardResult {
	return c.FetchRewards(slices.Values(rewardIds), ctx)
}

func (c *Client) FetchRewards(idIter iter.Seq[RewardId], ctx context.Context) <-chan RewardResult {
	jobs := make(chan int, c.MaxParallelism)
	rewardResults := make(chan RewardResult)
	wg := &sync.WaitGroup{}

	go func() {
		jobCounter := 1
		defer func() {
			wg.Wait()
			close(rewardResults)
		}()
		for id := range idIter {
			// Guard channel to limit parallelism
			jobs <- jobCounter
			if ctx.Err() != nil {
				break
			}
			jobCounter += 1
			wg.Add(1)
			go c.fetchRewardInternal(id, rewardResults, func() {
				<-jobs
				wg.Done()
			})
		}
	}()

	return rewardResults
}

func (c *Client) FetchReward(id RewardId) (*Reward, error) {
	reward := &RewardResponse{}
	err := c.fetch(id.ApiUrl(), reward)
	return &reward.Data, err
}

func (c *Client) FetchCampaign(id CampaignId) (*Campaign, error) {
	campaign := &CampaignResponse{}
	err := c.fetch(id.ApiUrl(), campaign)
	return &campaign.Data, err
}

func (c *Client) fetch(url *url.URL, target any) error {
	resp, err := c.httpClient.Get(url.String())
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return json.NewDecoder(resp.Body).Decode(target)
	case http.StatusNotFound:
		return &ResponseCodeError{StatusCode: resp.StatusCode, Message: fmt.Sprintf("URL not found: %s", url.String())}
	case http.StatusForbidden:
		return &ResponseCodeError{StatusCode: resp.StatusCode, Message: fmt.Sprintf("access forbidden for URL: %s", url.String())}
	case http.StatusTooManyRequests:
		return &ResponseCodeError{StatusCode: resp.StatusCode, Message: fmt.Sprintf("Hit rate limit for URL: %s", url.String())}
	default:
		return &ResponseCodeError{StatusCode: resp.StatusCode, Message: fmt.Sprintf("error fetching URL: %s", url.String())}
	}
}
