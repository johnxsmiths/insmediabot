package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"slmedia/pkg/config"
	"slmedia/pkg/telegram"
)

var (
	botOnce sync.Once
	botInst *telegram.Bot
	botCfg  *config.Config
)

// initBot initializes the Bot instance as a singleton across warm serverless invocations.
func initBot() {
	botCfg = config.Load()
	botInst = telegram.NewBot(botCfg)
}

// Handler is the primary entry point executed by Vercel Go Serverless Runtime.
func Handler(w http.ResponseWriter, r *http.Request) {
	botOnce.Do(initBot)

	// Route handling:
	// GET / or /healthz or /api/telegram -> Status & Webhook Auto-detection
	if r.Method == http.MethodGet {
		handleHealthCheck(w, r)
		return
	}

	// Only POST is expected for incoming Telegram Webhook updates
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST, GET")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Optional secret token validation
	if botCfg.TelegramWebhookSecret != "" {
		incomingSecret := r.Header.Get("X-Telegram-Bot-Api-Secret-Token")
		if incomingSecret != botCfg.TelegramWebhookSecret {
			log.Printf("[Security] Invalid secret token received")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	// Read and limit webhook body payload (Telegram max payload is well under 1MB)
	body, err := io.ReadAll(io.LimitReader(r.Body, 1024*1024))
	if err != nil {
		log.Printf("[Error] Failed reading request body: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if len(body) == 0 {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"empty_payload"}`))
		return
	}

	var update telegram.Update
	if err := json.Unmarshal(body, &update); err != nil {
		log.Printf("[Error] Failed parsing update json: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Set execution context timeout aligned with Vercel serverless bounds (50s)
	ctx, cancel := context.WithTimeout(r.Context(), 50*time.Second)
	defer cancel()

	// Process the update synchronously within the serverless invocation
	if err := botInst.ProcessUpdate(ctx, &update); err != nil {
		log.Printf("[Error] ProcessUpdate failed: %v", err)
	}

	// Return 200 OK immediately to acknowledge Telegram webhook
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	// Auto-detect public URL from request headers if deployed on Vercel
	scheme := "https"
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}
	host := r.Host
	if fwdHost := r.Header.Get("X-Forwarded-Host"); fwdHost != "" {
		host = fwdHost
	}

	webhookURL := fmt.Sprintf("%s://%s/api/telegram", scheme, host)

	// Check if setup parameter ?setup=true is called to auto-configure webhook
	if r.URL.Query().Get("setup") == "true" {
		if botCfg.TelegramBotToken == "" {
			http.Error(w, "TELEGRAM_BOT_TOKEN is not configured", http.StatusBadRequest)
			return
		}
		client := telegram.NewClient(botCfg.TelegramBotToken)
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		resp, err := client.SetWebhook(ctx, webhookURL, botCfg.TelegramWebhookSecret)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to set webhook: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":      "webhook_configured_successfully",
			"webhook_url": webhookURL,
			"telegram":    resp,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "online",
		"service":     "Instagram Downloader Telegram Bot",
		"runtime":     "Vercel Serverless Go",
		"detected_url": webhookURL,
		"setup_hint":  fmt.Sprintf("Visit %s?setup=true to automatically register this webhook with Telegram!", webhookURL),
		"providers":   botCfg.ProviderOrder,
		"languages":   12,
	})
}
