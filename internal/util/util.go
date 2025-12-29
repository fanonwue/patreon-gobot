package util

import (
	"strings"
	"time"

	"github.com/fanonwue/goutils"
)

const EmojiGreenCheck = "✅"
const EmojiCross = "❌"
const EnvPrefix = "PB_"

var envVarHelper = goutils.EnvVarHelper(EnvPrefix)

func TrimHtmlText(s string) string {
	return strings.Trim(s, "\n ")
}

func PrefixEnvVar(s string) string {
	return envVarHelper.PrefixVar(s)
}

func ToUTC(time *time.Time) *time.Time {
	if time == nil {
		return nil
	}
	utc := time.UTC()
	return &utc
}
