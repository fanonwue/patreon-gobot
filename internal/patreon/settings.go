package patreon

import "net/url"

const BaseUrlRaw = "https://www.patreon.com/"

var baseUrl, _ = url.Parse(BaseUrlRaw)
