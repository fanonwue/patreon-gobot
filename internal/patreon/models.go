package patreon

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type (
	RelationshipData struct {
		Id   int    `json:"id,string"`
		Type string `json:"type"`
	}
	RelationshipLinks struct {
		Related string `json:"related"`
	}

	Relationship struct {
		Data  RelationshipData  `json:"data"`
		Links RelationshipLinks `json:"links"`
	}

	RewardAttributes struct {
		AmountCents int       `json:"amount_cents"`
		CreatedAt   time.Time `json:"created_at"`
		EditedAt    time.Time `json:"edited_at"`
		PublishedAt time.Time `json:"published_at"`
		Published   bool      `json:"published"`
		Currency    string    `json:"currency"`
		Description string    `json:"description"`
		Remaining   int       `json:"remaining"`
		UserLimit   int       `json:"user_limit"`
		Title       string    `json:"title"`
		Url         string    `json:"url"`
		ImageUrl    string    `json:"image_url"`
	}

	RewardRelationships struct {
		Campaign Relationship `json:"campaign"`
	}

	Reward struct {
		Id            RewardId            `json:"id"`
		Attributes    RewardAttributes    `json:"attributes"`
		Relationships RewardRelationships `json:"relationships"`
	}

	RewardResponse struct {
		Data Reward `json:"data"`
	}

	CampaignAttributes struct {
		Name        string    `json:"name"`
		Url         string    `json:"url"`
		ImageUrl    string    `json:"image_url"`
		CreatedAt   time.Time `json:"created_at"`
		PublishedAt time.Time `json:"published_at"`
		Nsfw        bool      `json:"is_nsfw"`
	}

	Campaign struct {
		Id         CampaignId         `json:"id"`
		Type       string             `json:"type"`
		Attributes CampaignAttributes `json:"attributes"`
	}

	CampaignResponse struct {
		Data Campaign `json:"data"`
	}

	RewardId   int
	CampaignId int
)

func unmarshalId(buf []byte) (int, error) {
	var rawId string
	err := json.Unmarshal(buf, &rawId)
	if err != nil {
		return -1, err
	}
	parsedId, err := strconv.Atoi(rawId)
	if err != nil {
		return -1, err
	}
	return parsedId, nil
}

func (r *Reward) IsAvailable() bool {
	if r.Id == 7790866 {
		return true
	}
	return r.Attributes.Remaining > 0
}

func (r *Reward) CampaignId() (CampaignId, error) {
	id := CampaignId(r.Relationships.Campaign.Data.Id)
	if id == 0 {
		return id, errors.New("reward has no associated campaign")
	}
	return id, nil
}

func (r *Reward) CheckoutUrl() *url.URL {
	checkoutUrl, _ := baseUrl.Parse(r.Attributes.Url)
	return checkoutUrl
}

func (r *Reward) FullUrl() string {
	checkoutUrlWithoutLeadingSlash := strings.TrimLeft(r.Attributes.Url, "/")
	return BaseUrlRaw + checkoutUrlWithoutLeadingSlash
}

func (r *Reward) Title() string {
	return r.Attributes.Title
}

func (r *Reward) FormattedAmount() string {
	amount := fmt.Sprintf("%.2f", float64(r.Attributes.AmountCents)/100)
	return fmt.Sprintf("%s %s", amount, r.Attributes.Currency)
}

func (c *Campaign) FullUrl() string {
	return c.Attributes.Url
}

func (c *Campaign) Name() string {
	return c.Attributes.Name
}

func (id *RewardId) UnmarshalJSON(buf []byte) error {
	newId, err := unmarshalId(buf)
	if err != nil {
		return err
	}
	*id = RewardId(newId)
	return nil
}

func (id *CampaignId) UnmarshalJSON(buf []byte) error {
	newId, err := unmarshalId(buf)
	if err != nil {
		return err
	}
	*id = CampaignId(newId)
	return nil
}

func (id *RewardId) ApiUrl() *url.URL {
	apiUrl, _ := baseUrl.Parse("/api/rewards/" + strconv.Itoa(int(*id)))
	return apiUrl
}

func (id *CampaignId) ApiUrl() *url.URL {
	apiUrl, _ := baseUrl.Parse("/api/campaigns/" + strconv.Itoa(int(*id)))
	return apiUrl
}

func (id *RewardId) Compare(b *RewardId) int {
	return int(*id) - int(*b)
}

func (id *CampaignId) Compare(b *CampaignId) int {
	return int(*id) - int(*b)
}
