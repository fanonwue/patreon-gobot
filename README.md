# Patreon GoBot

This bot is a reimplementation of my Kotlin-based [PatreonRewardAvailabilityBot](https://github.com/fanonwue/PatreonRewardAvailabilityBot).

It's aim is to provide a way to get notified about Rewards with limited slots becoming
available again. It's polling https://www.patreon.com every few minutes (configurable) to
check the configured rewards. Notifications will be sent via a Telegram bot.

Please note that Patreon apparently started blocking IP addresses from hosting providers. As
this project uses the unauthenticated public API that the website uses as well, this restriction
applies here as well. If you receive `HTTP 403` responses, this is the reason.

Not affiliated in any way with Patreon.