package telegram

import (
	"github.com/fanonwue/patreon-gobot/internal/patreon"
	"github.com/fanonwue/patreon-gobot/internal/tmpl"
	"github.com/fanonwue/patreon-gobot/internal/util"
	"html/template"
)

var listRewardsTemplate = template.Must(createTemplate(tmpl.TemplatePath("list.gohtml")))
var missingRewardsTemplate = template.Must(createTemplate(tmpl.TemplatePath("missing-rewards.gohtml")))
var rewardAvailableTemplate = template.Must(createTemplate(tmpl.TemplatePath("reward-available.gohtml")))

var privacyPolicyTemplate = util.TrimHtmlText(`
This bot saves the following user information:

1. Your Chat ID (to identify you and match your data to your Telegram account)
	- In your case, this would be <code>%d</code>

2. Your provided user information:
	- Unread notes setting

3. Your FurAffinity cookies 
	- these are very sensitive, this allows the bot to fully impersonate you, which is required due to how FurAffinity works

4. A list of IDs that belong to your FurAffinity account: Note IDs, Comment IDs, Submission IDs and Journal IDs
	- this is needed to keep track of entries this bot has notified you about already. No content is stored, although it is fetched temporarily when notifying you.
`)

var baseTemplate = template.Must(
	template.New(tmpl.BaseTemplateName).Funcs(templateFuncMap()).ParseFS(tmpl.TemplateFS(), tmpl.TemplatePath(tmpl.BaseTemplateName)),
)

func createTemplate(targetTemplatePath string) (*template.Template, error) {
	cloned, err := baseTemplate.Clone()
	if err != nil {
		return nil, err
	}

	return cloned.ParseFS(tmpl.TemplateFS(), targetTemplatePath)
}

func templateFuncMap() template.FuncMap {
	return template.FuncMap{
		"rewardMissingReason": func(reason int) string {
			switch reason {
			case patreon.RewardErrorForbidden:
				return "Forbidden"
			case patreon.RewardErrorNotFound:
				return "Not Found"
			case patreon.RewardErrorNoCampaign:
				return "No Campaign"
			case patreon.RewardFound:
				return "Reward found (?!?!)"
			default:
				return "Unknown error"
			}
		},
	}
}
