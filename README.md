## Go website reachability checker

Performs scheduled HTTP HEAD request in order to check the reachability of the specified websites. Any occurring error
will be reported via telegram message. Therefore, the app needs a valid
[Telegram bot token](https://core.telegram.org/bots), and a Telegram user id in order to function properly.

### What the app does

1. Load data/config.json.
2. Start go routines for each individual website.
3. Report via Telegram if an error (!= HTTP Status 200) occurred.

### Usage

1. Copy `docker/data/config.json` to `data/config.json`
2. Adjust the config to your needs 
3. Run: `go run main.go`

### Usage with Docker

#### With docker compose

- Start the container: `docker compose up -d`
- Stop the container: `docker compose down`

#### Alternative without docker compose

1. Create the container with `make build`
2. Start the container with `make run`

### Example config

```
{
  "TelegramBotToken": "YOUR_TELEGRAM_BOT_TOKEN",
  "TelegramUserID": "YOUR_TELEGRAM_USER_ID",
  "ClientRequestTimeout": "5s",
  "Websites": [
    {
      "Name": "Google",
      "URL": "https://google.de",
      "Interval": "5m"
    },
    {
      "Name": "Yahoo",
      "URL": "https://yahoo.de",
      "Interval": "15m"
    }
  ]
}
```

