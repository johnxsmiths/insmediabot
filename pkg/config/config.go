package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// Config represents all application configuration parameters.
type Config struct {
	TelegramBotToken      string
	TelegramWebhookSecret string
	ProviderOrder         []string

	// Custom Provider URLs
	IGExportURL     string
	FastDLURL       string
	SSSInstagramURL string
	SnapSaveURL     string
	SaveClipURL     string
	SaveFromInsURL  string

	// Temporary Storage Configuration
	TempStorageProvider string
	TempStorageAPIURL   string
	TempStorageAPIKey   string
	TempFileTTL         int

	// Rate Limiter
	RateLimitRequests      int
	RateLimitWindowSeconds int
	LogLevel               string

	// Force Subscription Channel (e.g. @channelusername or -100xxxxxxx)
	ForceSubChannel     string
	ForceSubChannelLink string

	// Log / Backup Channel where media/actions are mirrored
	LogChannelID string

	// MongoDB Configuration
	MongoDBURI  string
	MongoDBName string

	// Owner ID(s) authorized for /broadcast and /stats
	OwnerIDs []int64
}

// Load reads configuration from the environment and optionally a local .env file.
func Load() *Config {
	loadDotEnv(".env")

	cfg := &Config{
		TelegramBotToken:      getEnv("TELEGRAM_BOT_TOKEN", ""),
		TelegramWebhookSecret: getEnv("TELEGRAM_WEBHOOK_SECRET", ""),
		ProviderOrder:         parseCommaSeparated(getEnv("PROVIDER_ORDER", "igexport,fastdl,sssinstagram,snapsave,saveclip,savefromins")),

		IGExportURL:     getEnv("IGEXPORT_API_URL", "https://igexport.com/api/ig-photo/"),
		FastDLURL:       getEnv("FASTDL_API_URL", "https://api-wh.fastdl.app/api/convert"),
		SSSInstagramURL: getEnv("SSSINSTAGRAM_API_URL", "https://api-wh.sssinstagram.com/api/convert"),
		SnapSaveURL:     getEnv("SNAPSAVE_API_URL", "https://snapsave.app/action.php?lang=en"),
		SaveClipURL:     getEnv("SAVECLIP_API_URL", "https://v3.saveclip.app/api/ajaxSearch"),
		SaveFromInsURL:  getEnv("SAVEFROMINS_API_URL", "https://api.savefromins.com/api/contentsite_api/media/parse"),

		TempStorageProvider: getEnv("TEMP_STORAGE_PROVIDER", "tmpfiles"),
		TempStorageAPIURL:   getEnv("TEMP_STORAGE_API_URL", "https://tmpfiles.org/api/v1/upload"),
		TempStorageAPIKey:   getEnv("TEMP_STORAGE_API_KEY", ""),
		TempFileTTL:         getEnvInt("TEMP_FILE_TTL", 3600),

		RateLimitRequests:      getEnvInt("RATE_LIMIT_REQUESTS", 15),
		RateLimitWindowSeconds: getEnvInt("RATE_LIMIT_WINDOW_SECONDS", 60),
		LogLevel:               getEnv("LOG_LEVEL", "info"),

		ForceSubChannel:     getEnv("FORCE_SUB_CHANNEL", ""),
		ForceSubChannelLink: getEnv("FORCE_SUB_CHANNEL_LINK", ""),
		LogChannelID:        getEnv("LOG_CHANNEL_ID", ""),

		MongoDBURI:  getEnv("MONGODB_URI", ""),
		MongoDBName: getEnv("MONGODB_NAME", "slmedia"),
		OwnerIDs:    parseCommaSeparatedInt64(getEnv("OWNER_ID", "")),
	}

	return cfg
}

// IsOwner returns true if the provided userID is in OwnerIDs.
func (c *Config) IsOwner(userID int64) bool {
	for _, id := range c.OwnerIDs {
		if id == userID {
			return true
		}
	}
	return false
}

func parseCommaSeparatedInt64(val string) []int64 {
	parts := strings.Split(val, ",")
	var result []int64
	for _, p := range parts {
		clean := strings.TrimSpace(p)
		if clean != "" {
			if id, err := strconv.ParseInt(clean, 10, 64); err == nil {
				result = append(result, id)
			}
		}
	}
	return result
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return strings.TrimSpace(val)
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
			return i
		}
	}
	return defaultVal
}

func parseCommaSeparated(val string) []string {
	parts := strings.Split(val, ",")
	var result []string
	for _, p := range parts {
		clean := strings.TrimSpace(p)
		if clean != "" {
			result = append(result, strings.ToLower(clean))
		}
	}
	if len(result) == 0 {
		return []string{"igexport", "fastdl", "sssinstagram", "snapsave", "saveclip", "savefromins"}
	}
	return result
}

func loadDotEnv(filepath string) {
	f, err := os.Open(filepath)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			val = strings.Trim(val, `"'`)
			if os.Getenv(key) == "" {
				_ = os.Setenv(key, val)
			}
		}
	}
}
