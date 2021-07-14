package main

import (
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/yanzay/tbot/v2"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"time"
)

const (
	defaultRequestTimout = "15s" // Set the default request timout
)

var (
	config     AppConfig
	tClient    *tbot.Client
	httpClient http.Client
)

// AppConfig
type AppConfig struct {
	TelegramBotToken     string     // Token of the telegram bot in use
	TelegramUserID       string     // Telegram user ID of the user, who should receive the notifications
	ClientRequestTimeout string     // Client request timeout in as a duration string (e.g. 5s or 10m). Timout of 0 means no timeout
	Websites             []WSConfig // An array of WSConfig
}

// Website config
type WSConfig struct {
	Name     string // Name of the WSConfig
	URL      string // URL under which the Website should be available
	Interval string // Duration string (e.g. 5s or 10m)
}

func init() {
	// Initialize zerolog to use pretty logging and RFC3339 time format
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
}

func main() {
	var termSig chan os.Signal
	termSig = make(chan os.Signal, 1)
	signal.Notify(termSig, os.Interrupt)

	content, err := ioutil.ReadFile("./data/config.json")
	if err != nil {
		log.Fatal().Msg(fmt.Sprintf("Error when opening file: %s", err))
	}

	// Now let's unmarshall the data into `config`
	err = json.Unmarshal(content, &config)
	if err != nil {
		panic(err)
	}

	if config.TelegramBotToken == "" {
		log.Fatal().Msg(fmt.Sprintf("Missing telegram bot token"))
	}
	if config.TelegramUserID == "" {
		log.Fatal().Msg(fmt.Sprintf("Missing telegram user id"))
	}

	// Create a telegram client
	tClient = tbot.New(config.TelegramBotToken).Client()

	if config.ClientRequestTimeout == "" {
		config.ClientRequestTimeout = defaultRequestTimout
	}

	timeout, err := time.ParseDuration(config.ClientRequestTimeout)
	if err != nil {
		panic(err)
	}
	// Create a http client with custom timeout
	httpClient = http.Client{
		Timeout: timeout, // Timeout of zero means no timeout
	}

	for _, ws := range config.Websites {
		go checkWebsite(ws)
	}

	// Wait until CTRL-C is pressed
	<-termSig
}

// checkWebsite Makes a HTTP HEAD request in order to verify if the website is reachable
func checkWebsite(ws WSConfig) {
	checkInterval, err := time.ParseDuration(ws.Interval)
	if err != nil {
		panic(err)
	}
	for {
		resp, err := httpClient.Head(ws.URL)
		if err != nil {
			// Send error message
			errMsg := fmt.Sprintf("Website %q (%s) is down!\nPlease take immediate action.", ws.Name, ws.URL)
			log.Err(err).Msg(fmt.Sprintf(errMsg))
			sendTelegramMessage(errMsg)
		} else {
			// Everything ok
			if resp.StatusCode == http.StatusOK {
				log.Info().Msg(fmt.Sprintf("Website %q (%s) ok! -> Next check at %s\n", ws.Name, ws.URL, time.Now().Add(checkInterval)))
			} else {
				// Send error message
				errMsg := fmt.Sprintf("Website %q (%s) is down!\nStatusCode: %d\nStatus: %s\nPlease take immediate action.", ws.Name, ws.URL, resp.StatusCode, resp.Status)
				log.Error().Msg(fmt.Sprintf(errMsg))
				sendTelegramMessage(errMsg)
			}
		}
		time.Sleep(checkInterval)
	}
}

// sendTelegramMessage Sends a telegram message to the user which is specified in the config via TelegramUserID
func sendTelegramMessage(message string) {
	message = "Website reachability: " + message
	_, err := tClient.SendMessage(config.TelegramUserID, message)
	if err != nil {
		log.Err(err).Msg("Cannot send telegram message")
	}
}
