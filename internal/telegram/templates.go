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
	- Language

3. Your tracked Patreon rewards (their IDs)
	- These will be periodically checked via the Patreon API to see whether new slots are available
	- This can be linked to the campaign and the creator they are associated with
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
		"rewardMissingReason": func(reason patreon.RewardStatus) string {
			return reason.Text()
		},
		"tgEscape": func(s string) string { return Escape(s) },
	}
}
