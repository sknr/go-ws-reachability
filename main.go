package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/yanzay/tbot/v2"
)

const (
	defaultRequestTimeout = 15 * time.Second
)

// AppConfig structure matching the JSON structure
type AppConfig struct {
	TelegramBotToken     string     `json:"TelegramBotToken"`
	TelegramUserID       string     `json:"TelegramUserID"`
	ClientRequestTimeout string     `json:"ClientRequestTimeout"`
	Websites             []WSConfig `json:"Websites"`
}

// WSConfig representing a monitored website
type WSConfig struct {
	Name     string `json:"Name"`
	URL      string `json:"URL"`
	Interval string `json:"Interval"`
}

// App encapsulates the application dependencies and state
type App struct {
	config     AppConfig
	tClient    *tbot.Client
	httpClient *http.Client
	sendAlert  func(message string)
}

func init() {
	// Initialize zerolog to use pretty logging and RFC3339 time format
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
}

func main() {
	// Setup context that cancels on interrupt or term signals (SIGINT, SIGTERM)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Load configuration
	configPath := "./data/config.json"
	content, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to read config file")
	}

	var cfg AppConfig
	if err := json.Unmarshal(content, &cfg); err != nil {
		log.Fatal().Err(err).Msg("Failed to unmarshal config JSON")
	}

	// Environment variable overrides
	if envToken := os.Getenv("TELEGRAM_BOT_TOKEN"); envToken != "" {
		cfg.TelegramBotToken = envToken
	}
	if envUserID := os.Getenv("TELEGRAM_USER_ID"); envUserID != "" {
		cfg.TelegramUserID = envUserID
	}

	// Validate config
	if cfg.TelegramBotToken == "" {
		log.Fatal().Msg("Missing telegram bot token")
	}
	if cfg.TelegramUserID == "" {
		log.Fatal().Msg("Missing telegram user id")
	}

	// Determine client request timeout
	var timeout time.Duration
	if cfg.ClientRequestTimeout == "" {
		timeout = defaultRequestTimeout
	} else {
		t, err := time.ParseDuration(cfg.ClientRequestTimeout)
		if err != nil {
			log.Fatal().Err(err).Msgf("Invalid ClientRequestTimeout: %q", cfg.ClientRequestTimeout)
		}
		timeout = t
	}

	// Validate website configurations
	for _, ws := range cfg.Websites {
		if ws.Name == "" {
			log.Fatal().Msg("Website name cannot be empty")
		}
		if _, err := url.ParseRequestURI(ws.URL); err != nil {
			log.Fatal().Err(err).Msgf("Invalid website URL: %q for %q", ws.URL, ws.Name)
		}
		if _, err := time.ParseDuration(ws.Interval); err != nil {
			log.Fatal().Err(err).Msgf("Invalid check interval %q for %q", ws.Interval, ws.Name)
		}
	}

	// Instantiate application
	app := &App{
		config:  cfg,
		tClient: tbot.New(cfg.TelegramBotToken).Client(),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
	app.sendAlert = app.sendTelegramMessage

	log.Info().Msg("Starting website reachability checker...")

	var wg sync.WaitGroup
	for _, ws := range cfg.Websites {
		wg.Add(1)
		go func(w WSConfig) {
			defer wg.Done()
			app.checkWebsite(ctx, w)
		}(ws)
	}

	// Wait until context is cancelled (via OS signals)
	<-ctx.Done()
	log.Info().Msg("Shutdown signal received. Waiting for goroutines to finish...")

	// Wait for all website checks to stop cleanly
	wg.Wait()
	log.Info().Msg("Application shut down gracefully.")
}

// checkWebsite Makes a HTTP HEAD request in order to verify if the website is reachable
func (app *App) checkWebsite(ctx context.Context, ws WSConfig) {
	checkInterval, _ := time.ParseDuration(ws.Interval) // Already validated at startup
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	// Track website state. Assume UP (false) initially.
	isDown := false

	// Helper function to run check
	runCheck := func() {
		// Create HTTP HEAD request with context to allow cancellation
		req, err := http.NewRequestWithContext(ctx, http.MethodHead, ws.URL, nil)
		if err != nil {
			log.Error().Err(err).Msgf("[%s] Failed to create request", ws.Name)
			return
		}

		resp, err := app.httpClient.Do(req)
		if err != nil {
			// Request failed (e.g. timeout, connection refused)
			if !isDown {
				isDown = true
				errMsg := fmt.Sprintf("Website %q (%s) is down!\nError: %s\nPlease take immediate action.", ws.Name, ws.URL, err.Error())
				log.Error().Err(err).Msgf("[%s] Website went down", ws.Name)
				app.sendAlert(errMsg)
			}
			return
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			// Status code is not 200 OK
			if !isDown {
				isDown = true
				errMsg := fmt.Sprintf("Website %q (%s) is down!\nStatusCode: %d\nStatus: %s\nPlease take immediate action.", ws.Name, ws.URL, resp.StatusCode, resp.Status)
				log.Error().Msgf("[%s] Website went down (Status %d)", ws.Name, resp.StatusCode)
				app.sendAlert(errMsg)
			}
		} else {
			// Status is 200 OK
			if isDown {
				isDown = false
				recoveryMsg := fmt.Sprintf("Website %q (%s) has recovered and is OK!", ws.Name, ws.URL)
				log.Info().Msgf("[%s] Website recovered", ws.Name)
				app.sendAlert(recoveryMsg)
			} else {
				log.Info().Msgf("[%s] Website OK! -> Next check in %s", ws.Name, checkInterval)
			}
		}
	}

	// Run initial check immediately
	runCheck()

	for {
		select {
		case <-ctx.Done():
			log.Debug().Msgf("[%s] Stopping monitor goroutine", ws.Name)
			return
		case <-ticker.C:
			runCheck()
		}
	}
}

// sendTelegramMessage Sends a telegram message to the user which is specified in the config via TelegramUserID
func (app *App) sendTelegramMessage(message string) {
	fullMessage := "Website reachability: " + message
	_, err := app.tClient.SendMessage(app.config.TelegramUserID, fullMessage)
	if err != nil {
		log.Err(err).Msg("Cannot send telegram message")
	}
}
