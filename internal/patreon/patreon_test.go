package patreon

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fanonwue/patreon-gobot/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var root = "../../"

var rewardsPathRegex = regexp.MustCompile("^/api/rewards/(\\d+)")
var campaignsPathRegex = regexp.MustCompile("^/api/campaigns/(\\d+)")
var forbiddenRewards = []RewardId{1000, 2000, 3000}
var availableRewards = []RewardId{7790866}
var server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if rewardsPathRegex.MatchString(r.URL.Path) {
		matchedId := rewardsPathRegex.FindStringSubmatch(r.URL.Path)
		if len(matchedId) <= 1 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		rawId, err := strconv.Atoi(matchedId[1])
		if err != nil || rawId == 0 {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		id := RewardId(rawId)

		if slices.Contains(forbiddenRewards, id) {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		writeStub(fmt.Sprintf("test/stubs/rewards/%d.json", id), w)
		return
	}

	if campaignsPathRegex.MatchString(r.URL.Path) {
		matchedId := campaignsPathRegex.FindStringSubmatch(r.URL.Path)
		if len(matchedId) <= 1 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		rawId, err := strconv.Atoi(matchedId[1])
		if err != nil || rawId == 0 {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		writeStub(fmt.Sprintf("test/stubs/campaigns/%d.json", rawId), w)
	}
}))

func writeStub(path string, w http.ResponseWriter) {
	f, err := os.Open(root + path)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	defer f.Close()
	w.WriteHeader(http.StatusOK)
	_, _ = f.WriteTo(w)
}

func setup(t *testing.T) func(*testing.T) {
	originalBaseUrl := baseUrl
	baseUrl, _ = url.Parse(server.URL)
	return func(t *testing.T) {
		baseUrl = originalBaseUrl
	}
}

func TestClient_FetchReward(t *testing.T) {
	teardown := setup(t)
	defer teardown(t)

	client := NewClient(1)
	client.httpClient = server.Client()

	rewardTestMap := make(map[RewardId]func(id RewardId, r *Reward, err error))

	rewardTestMap[10206990] = func(id RewardId, r *Reward, err error) {
		assert.NoError(t, err)
		assert.Equal(t, id, r.Id)
		assert.Equal(t, 3876079, r.Relationships.Campaign.Data.Id)
		assert.False(t, r.IsAvailable())
		assert.Equal(t, 6000, r.Attributes.AmountCents)
		assert.Equal(t, "Disciple", r.Attributes.Title)
		assert.Equal(t, "60.00 $", r.FormattedAmount())
		assert.Equal(t, util.Currency("USD"), r.Attributes.Currency)

		expectedCreatedAt, _ := time.Parse(time.RFC3339, "2023-09-19T22:44:20.842+00:00")
		assert.Equal(t, expectedCreatedAt, r.Attributes.CreatedAt)
		expectedCheckout, _ := url.Parse("/checkout/NommzArts?rid=10206990")
		assert.Equal(t, expectedCheckout.Path, r.CheckoutUrl().Path)

	}

	rewardTestMap[availableRewards[0]] = func(id RewardId, r *Reward, err error) {
		assert.NoError(t, err)
		assert.Equal(t, id, r.Id)
		assert.Zero(t, r.Relationships.Campaign.Data.Id)
		assert.Zero(t, r.Relationships.Campaign.Data.Type)
		assert.True(t, r.IsAvailable())
		assert.Equal(t, 2000, r.Attributes.AmountCents)
		assert.Equal(t, 10, r.Attributes.UserLimit)

		expectedEditedAt, _ := time.Parse(time.RFC3339, "2023-08-30T10:17:37.341+00:00")
		assert.Equal(t, expectedEditedAt, r.Attributes.EditedAt)

	}

	rewardTestMap[1] = func(id RewardId, r *Reward, err error) {
		assert.Error(t, err, "reward should return error")
		assert.ErrorContains(t, err, "received status 404")
	}

	rewardTestMap[1000] = func(id RewardId, r *Reward, err error) {
		assert.Error(t, err, "reward should return error")
		assert.ErrorContains(t, err, "received status 403")
	}

	for key, callback := range rewardTestMap {
		reward, err := client.FetchReward(key, true)
		callback(key, reward, err)
	}
}

func TestClient_FetchCampaign(t *testing.T) {
	teardown := setup(t)
	defer teardown(t)

	client := NewClient(1)
	client.httpClient = server.Client()

	campaignTestMap := make(map[CampaignId]func(id CampaignId, c *Campaign, err error))

	campaignTestMap[3876079] = func(id CampaignId, c *Campaign, err error) {
		assert.NoError(t, err)
		assert.Equal(t, id, c.Id)
		assert.Equal(t, "NommzArts", c.Attributes.Name)
		assert.Equal(t, "https://www.patreon.com/NommzArts", c.Attributes.Url)
		assert.True(t, c.Attributes.Nsfw)

		expectedCreatedAt, _ := time.Parse(time.RFC3339, "2020-01-31T09:27:17.000+00:00")
		assert.Equal(t, expectedCreatedAt, c.Attributes.CreatedAt)
	}

	campaignTestMap[173646] = func(id CampaignId, c *Campaign, err error) {
		assert.NoError(t, err)
		assert.Equal(t, id, c.Id)
		assert.Equal(t, "Futon", c.Attributes.Name)
		assert.False(t, c.Attributes.Nsfw)

		expectedPublishedAt, _ := time.Parse(time.RFC3339, "2019-04-28T00:02:12.000+00:00")
		assert.Equal(t, expectedPublishedAt, c.Attributes.PublishedAt)
	}

	campaignTestMap[1] = func(id CampaignId, c *Campaign, err error) {
		assert.Error(t, err, "campaign should return error")
		assert.ErrorContains(t, err, "received status 404")
	}

	for key, callback := range campaignTestMap {
		campaign, err := client.FetchCampaign(key, true)
		callback(key, campaign, err)
	}
}

func TestClient_FetchRewardsSlice(t *testing.T) {
	teardown := setup(t)
	defer teardown(t)

	client := NewClient(1)
	client.httpClient = server.Client()

	rewardIds := slices.Concat([]RewardId{1, 7790866, 10206990}, forbiddenRewards)

	rewardResults := client.FetchRewardsSlice(rewardIds, true, context.Background())

	for r := range rewardResults {
		if r.Id == 1 {
			assert.False(t, r.IsPresent())
			assert.Equal(t, r.Status, RewardErrorNotFound)
		} else if slices.Contains(forbiddenRewards, r.Id) {
			assert.False(t, r.IsPresent())
			assert.Equal(t, r.Status, RewardErrorForbidden)
		} else {
			assert.True(t, r.IsPresent())
			assert.Equal(t, r.Status, RewardFound)
			assert.Equal(t, r.Id, r.Reward.Id)

			if slices.Contains(availableRewards, r.Id) {
				assert.Greater(t, r.Reward.Attributes.Remaining, 0)
				assert.True(t, r.IsAvailable())
			}
		}
	}
}

func TestIdUnmarshal(t *testing.T) {
	tests := []struct {
		name  string
		input string
		out   any
		want  any
	}{
		{name: "reward ID", input: `"10206990"`, out: new(RewardId), want: RewardId(10206990)},
		{name: "campaign ID", input: `"3876079"`, out: new(CampaignId), want: CampaignId(3876079)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, json.UnmarshalRead(strings.NewReader(tt.input), tt.out))
			assert.Equal(t, tt.want, indirectValue(tt.out))
		})
	}
}

func TestIdUnmarshalRejectsInvalidValues(t *testing.T) {
	for _, input := range []string{`123`, `"not-an-ID"`, `null`} {
		t.Run(input, func(t *testing.T) {
			id := RewardId(42)
			assert.Error(t, json.UnmarshalRead(strings.NewReader(input), &id))
			assert.Equal(t, RewardId(42), id)
		})
	}
}

func TestClientFetchRejectsTrailingJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data": {}} {}`))
	}))
	t.Cleanup(server.Close)

	endpoint, err := url.Parse(server.URL)
	require.NoError(t, err)
	client := NewClient(1)
	client.httpClient = server.Client()

	err = client.fetch(endpoint, &RewardResponse{})
	assert.Error(t, err)
}

func TestRewardHelpers(t *testing.T) {
	reward := Reward{Attributes: RewardAttributes{AmountCents: 1234, Currency: util.Currency("EUR"), Remaining: 1, Title: "Tier", Url: "/checkout/tier"}}

	assert.True(t, reward.IsAvailable())
	assert.Equal(t, "12.34 €", reward.FormattedAmount())
	assert.Equal(t, "Tier", reward.Title())
	assert.Equal(t, "/checkout/tier", reward.CheckoutUrl().Path)
	assert.Equal(t, "https://www.patreon.com/checkout/tier", reward.FullUrl())
	assert.EqualError(t, func() error { _, err := reward.CampaignId(); return err }(), "reward has no associated campaign")
}

func indirectValue(value any) any {
	switch id := value.(type) {
	case *RewardId:
		return *id
	case *CampaignId:
		return *id
	default:
		return value
	}
}
