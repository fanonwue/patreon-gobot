package patreon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"net/url"
	"slices"
	"sync"

	"github.com/fanonwue/goutils/logging"
)

type RewardStatus int

const (
	RewardFound RewardStatus = iota
	RewardErrorUnknown
	RewardErrorForbidden
	RewardErrorNotFound
	RewardErrorNoCampaign
	RewardErrorRateLimit
	RewardErrorInternalServerError
	RewardErrorGatewayError
)

func (rs RewardStatus) Text() string {
	switch rs {
	case RewardErrorForbidden:
		return "Forbidden"
	case RewardErrorNotFound:
		return "Not Found"
	case RewardErrorNoCampaign:
		return "No Campaign"
	case RewardErrorRateLimit:
		return "Rate limited"
	case RewardErrorInternalServerError:
		return "Internal Server Error (at Patreon)"
	case RewardErrorGatewayError:
		return "Gateway Error (at Patreon)"
	case RewardFound:
		return "Reward found (?!?!)"
	default:
		return "Unknown error"
	}
}

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:131.0) Gecko/20100101 Firefox/131.0"

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
		for rewardResult := range c.FetchRewardsSlice(rewardIds, true, ctx) {
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

func (c *Client) fetchRewardInternal(id RewardId, rewardChannel chan<- RewardResult, forceRefresh bool, callback func()) {
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
	reward, err := c.FetchReward(id, forceRefresh)

	if err == nil {
		ra.Reward = reward
		putInChannel = true
	} else {
		var responseCodeError *ResponseCodeError
		if errors.As(err, &responseCodeError) {
			putInChannel = true
			switch responseCodeError.StatusCode {
			case http.StatusForbidden:
				ra.Status = RewardErrorForbidden
			case http.StatusNotFound:
				ra.Status = RewardErrorNotFound
			case http.StatusTooManyRequests:
				ra.Status = RewardErrorRateLimit
			case http.StatusInternalServerError:
				ra.Status = RewardErrorInternalServerError
			case http.StatusGatewayTimeout, http.StatusBadGateway:
				ra.Status = RewardErrorGatewayError
			default:
				ra.Status = RewardErrorUnknown
				logging.Errorf("unknown error fetching reward: %v", err)
			}
		} else {
			logging.Errorf("error fetching reward %d: %v", id, err)
		}
	}
}

func (c *Client) FetchRewardsSlice(rewardIds []RewardId, forceRefresh bool, ctx context.Context) <-chan RewardResult {
	return c.FetchRewards(slices.Values(rewardIds), forceRefresh, ctx)
}

func (c *Client) FetchRewards(idIter iter.Seq[RewardId], forceRefresh bool, ctx context.Context) <-chan RewardResult {
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
			go c.fetchRewardInternal(id, rewardResults, forceRefresh, func() {
				<-jobs
				wg.Done()
			})
		}
	}()

	return rewardResults
}

func (c *Client) FetchReward(id RewardId, forceRefresh bool) (*Reward, error) {
	if cacheEnabled && !forceRefresh {
		cached, found := rewardsCache.Get(id)
		if found && cached != nil {
			return cached, nil
		}
	}
	logging.Debugf("Fetching reward %d", id)
	reward := &RewardResponse{}
	err := c.fetch(id.ApiUrl(), reward)
	var rewardData *Reward
	if err == nil && reward != nil {
		rewardData = &reward.Data
		// Make sure the reward actually got found before caching it
		if cacheEnabled && rewardData.Id != 0 {
			rewardsCache.Set(id, rewardData)
		}
	}
	return rewardData, err
}

func (c *Client) FetchCampaign(id CampaignId, forceRefresh bool) (*Campaign, error) {
	if cacheEnabled && !forceRefresh {
		cached, found := campaignsCache.Get(id)
		if found && cached != nil {
			return cached, nil
		}
	}
	logging.Debugf("Fetching campaign %d", id)
	campaign := &CampaignResponse{}
	err := c.fetch(id.ApiUrl(), campaign)
	var campaignData *Campaign
	if err == nil && campaign != nil {
		campaignData = &campaign.Data
		// Make sure the campaign actually got found before caching it
		if cacheEnabled && campaignData.Id != 0 {
			campaignsCache.Set(id, campaignData)
		}
	}
	return campaignData, err
}

//goland:noinspection GoBoolExpressions
func (c *Client) fetch(url *url.URL, target any) error {
	req, _ := http.NewRequest("GET", url.String(), nil)
	if userAgent != "" {
		req.Header.Add("User-Agent", userAgent)
	}
	resp, err := c.httpClient.Do(req)
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
		return &ResponseCodeError{StatusCode: resp.StatusCode, Message: fmt.Sprintf("hit rate limit for URL: %s", url.String())}
	default:
		return &ResponseCodeError{StatusCode: resp.StatusCode, Message: fmt.Sprintf("unknown error fetching URL: %s", url.String())}
	}
}
