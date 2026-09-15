package telegram

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"slmedia/pkg/config"
	"slmedia/pkg/db"
	"slmedia/pkg/i18n"
	"slmedia/pkg/media"
	"slmedia/pkg/ratelimit"
	"slmedia/pkg/resolver"
	"slmedia/pkg/storage"
)

// Bot orchestrates the serverless webhook lifecycle.
type Bot struct {
	cfg         *config.Config
	client      *Client
	resolver    *resolver.Manager
	storage     storage.Storage
	downloader  *media.Downloader
	rateLimiter *ratelimit.Limiter
	db          *db.DB

	botUsername     string
	botName         string
	botUsernameOnce sync.Once
}

// NewBot initializes the Bot with all pluggable components.
func NewBot(cfg *config.Config) *Bot {
	client := NewClient(cfg.TelegramBotToken)
	resManager := resolver.NewManager(cfg)
	tmpStorage := storage.NewTmpFilesStorage(cfg.TempStorageAPIURL)
	downloader := media.NewDownloader(50 * 1024 * 1024)
	limiter := ratelimit.New(cfg.RateLimitRequests, cfg.RateLimitWindowSeconds)

	database, err := db.Init(cfg.MongoDBURI, cfg.MongoDBName)
	if err != nil {
		log.Printf("[MongoDB] Failed to connect: %v", err)
	}

	return &Bot{
		cfg:         cfg,
		client:      client,
		resolver:    resManager,
		storage:     tmpStorage,
		downloader:  downloader,
		rateLimiter: limiter,
		db:          database,
	}
}

func (b *Bot) getBotInfo(ctx context.Context) (name string, username string) {
	b.botUsernameOnce.Do(func() {
		if me, err := b.client.GetMe(ctx); err == nil && me != nil {
			if me.FirstName != "" {
				b.botName = me.FirstName
			}
			if me.Username != "" {
				b.botUsername = me.Username
			}
			log.Printf("[Bot] Resolved bot identity: %s (@%s)", b.botName, b.botUsername)
		}
	})
	name = b.botName
	if name == "" {
		name = "Instagram Downloader"
	}
	username = b.botUsername
	if username == "" {
		username = "InstagramDownloader"
	}
	return name, username
}

func (b *Bot) getBotUsername(ctx context.Context) string {
	_, username := b.getBotInfo(ctx)
	return "@" + username
}

// checkForceSub checks if user is subscribed to the required channel.
func (b *Bot) checkForceSub(ctx context.Context, userID int64) bool {
	if b.cfg.ForceSubChannel == "" {
		return true
	}
	status, err := b.client.GetChatMember(ctx, b.cfg.ForceSubChannel, userID)
	if err != nil {
		log.Printf("[ForceSub] Check failed for user %d: %v", userID, err)
		// Fail open if channel configuration or permissions have an issue
		return true
	}
	// Valid Telegram statuses for active members
	return status == "member" || status == "administrator" || status == "creator"
}

// ProcessUpdate handles one incoming webhook update.
func (b *Bot) ProcessUpdate(ctx context.Context, update *Update) error {
	if update == nil {
		return nil
	}

	b.trackUserFromUpdate(update)

	// 1. Handle Inline Keyboard Button Callbacks
	if update.CallbackQuery != nil {
		return b.handleCallbackQuery(ctx, update.CallbackQuery)
	}

	if update.InlineQuery != nil {
		return b.handleInlineQuery(ctx, update.InlineQuery)
	}

	// 3. Handle Text Messages
	if update.Message != nil && update.Message.Text != "" {
		return b.handleTextMessage(ctx, update.Message)
	}

	return nil
}

func (b *Bot) trackUserFromUpdate(update *Update) {
	if b.db == nil {
		return
	}
	var u *User
	if update.Message != nil && update.Message.From != nil {
		u = update.Message.From
	} else if update.CallbackQuery != nil {
		u = &update.CallbackQuery.From
	} else if update.InlineQuery != nil {
		u = &update.InlineQuery.From
	}

	if u != nil && !u.IsBot {
		go func(user User) {
			dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			savedLang, _, err := b.db.UpsertUser(dbCtx, user.ID, user.Username, user.FirstName, user.LastName, user.LanguageCode)
			if err == nil && savedLang != "" {
				i18n.GlobalUserLangStore.Set(user.ID, savedLang)
			}
		}(*u)
	}
}

func (b *Bot) handleTextMessage(ctx context.Context, msg *Message) error {
	chatID := msg.Chat.ID
	userID := msg.From.ID
	text := strings.TrimSpace(msg.Text)

	if b.db != nil {
		if banned, reason := b.db.IsUserBanned(ctx, userID); banned {
			banMsg := "⛔ <b>You are banned from using this bot.</b>"
			if reason != "" {
				banMsg += fmt.Sprintf("\nReason: <i>%s</i>", reason)
			}
			_, err := b.client.SendMessage(ctx, chatID, banMsg, nil)
			return err
		}

		go func(uid, cid, mid int64, t string) {
			dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = b.db.LogMessage(dbCtx, uid, cid, mid, t)
		}(userID, chatID, msg.MessageID, text)
	}

	t := i18n.ForUser(userID)

	// Command routing
	switch {
	case strings.HasPrefix(text, "/start"):
		parts := strings.Fields(text)
		if len(parts) > 1 && strings.HasPrefix(parts[1], "dl_") {
			shortcode := strings.TrimPrefix(parts[1], "dl_")
			reconstructURL := "https://www.instagram.com/p/" + shortcode + "/"
			return b.processInstagramDownload(ctx, chatID, userID, reconstructURL)
		}
		return b.sendStartMessage(ctx, chatID, userID)
	case strings.HasPrefix(text, "/help"):
		_, err := b.client.SendMessage(ctx, chatID, b.getHelpMessage(ctx, userID), b.getHelpKeyboard())
		return err
	case strings.HasPrefix(text, "/about"):
		_, err := b.client.SendMessage(ctx, chatID, b.getAboutMessage(ctx, userID), b.getAboutKeyboard())
		return err
	case strings.HasPrefix(text, "/lang") || strings.HasPrefix(text, "/language"):
		return b.sendLanguageSelectorInNewMessage(ctx, chatID, userID)
	case strings.HasPrefix(text, "/stats"):
		return b.handleStatsCommand(ctx, chatID, userID)
	case strings.HasPrefix(text, "/broadcast"):
		return b.handleBroadcastCommand(ctx, msg)
	case strings.HasPrefix(text, "/ban"):
		return b.handleBanCommand(ctx, chatID, userID, text)
	case strings.HasPrefix(text, "/unban"):
		return b.handleUnbanCommand(ctx, chatID, userID, text)
	}

	// Check if message is or contains an Instagram URL
	words := strings.Fields(text)
	var targetURL string
	for _, w := range words {
		if resolver.IsInstagramURL(w) {
			targetURL = w
			break
		}
	}

	if targetURL == "" {
		_, err := b.client.SendMessage(ctx, chatID, t.InvalidURL, b.getMainKeyboard(userID))
		return err
	}

	// ForceSub Check: Must be subscribed to configured channel before downloading
	if !b.checkForceSub(ctx, userID) {
		return b.sendForceSubMessage(ctx, chatID, userID)
	}

	// Rate limiting verification
	if !b.rateLimiter.Allow(userID) {
		_, err := b.client.SendMessage(ctx, chatID, t.RateLimited, nil)
		return err
	}

	return b.processInstagramDownload(ctx, chatID, userID, targetURL)
}

func (b *Bot) processInstagramDownload(ctx context.Context, chatID, userID int64, rawURL string) error {
	t := i18n.ForUser(userID)

	cleanURL, err := resolver.NormalizeInstagramURL(rawURL)
	if err != nil {
		cleanURL = rawURL
	}

	// Step 1: Send Status Message
	b.client.SendChatAction(ctx, chatID, "typing")
	statusMsgID, err := b.client.SendMessage(ctx, chatID, t.ProcessingURL, nil)
	if err != nil {
		log.Printf("[Bot] Send initial status error: %v", err)
	}

	// Step 2: Resolve via Fallback Chain (Only 1 provider succeeds, no duplicate uploads)
	_ = b.client.EditMessageText(ctx, chatID, statusMsgID, t.ResolvingMedia, nil)

	mediaRes, err := b.resolver.Resolve(ctx, cleanURL)
	if err != nil || mediaRes == nil || len(mediaRes.Items) == 0 {
		log.Printf("[Bot] Resolver failed for %s: %v", cleanURL, err)
		if b.db != nil {
			go func(uid int64, u string) {
				dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = b.db.LogDownload(dbCtx, uid, u, "failed")
			}(userID, cleanURL)
		}
		fallbackText := fmt.Sprintf("%s\n\n%s <a href=\"%s\">Instagram</a>", t.DownloadError, t.BtnDirectLink, cleanURL)
		directKB := &InlineKeyboardMarkup{
			InlineKeyboard: [][]InlineKeyboardButton{
				{{Text: t.BtnDirectLink, URL: cleanURL}},
			},
		}
		_ = b.client.EditMessageText(ctx, chatID, statusMsgID, fallbackText, directKB)
		return nil
	}

	// Step 3: Media Upload Delivery
	_ = b.client.EditMessageText(ctx, chatID, statusMsgID, t.UploadingMedia, nil)

	uploadErr := b.deliverMedia(ctx, chatID, userID, mediaRes)
	if uploadErr != nil {
		log.Printf("[Bot] Delivery error: %v", uploadErr)
		if b.db != nil {
			go func(uid int64, u string) {
				dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = b.db.LogDownload(dbCtx, uid, u, "failed")
			}(userID, cleanURL)
		}
		fallbackText := fmt.Sprintf("%s\n\n🔗 %s", t.UploadFallback, cleanURL)
		directKB := &InlineKeyboardMarkup{
			InlineKeyboard: [][]InlineKeyboardButton{
				{{Text: t.BtnDirectLink, URL: cleanURL}},
			},
		}
		_ = b.client.EditMessageText(ctx, chatID, statusMsgID, fallbackText, directKB)
		return nil
	}

	if b.db != nil {
		go func(uid int64, u string) {
			dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = b.db.LogDownload(dbCtx, uid, u, "success")
		}(userID, cleanURL)
	}

	// Step 4: Clean up status message immediately after media delivery
	if statusMsgID > 0 {
		_ = b.client.DeleteMessage(ctx, chatID, statusMsgID)
	}

	// Step 5: Mirror to Log Channel if configured
	if b.cfg.LogChannelID != "" {
		go func() {
			logCtx, logCancel := context.WithTimeout(context.Background(), 40*time.Second)
			defer logCancel()
			_ = b.forwardToLogChannel(logCtx, userID, cleanURL, mediaRes)
		}()
	}

	return nil
}

func (b *Bot) deliverMedia(ctx context.Context, chatID, userID int64, res *resolver.MediaResult) error {
	caption := fmt.Sprintf("✨ Downloaded via %s", b.getBotUsername(ctx))

	// Case 1: Carousel / Album
	if len(res.Items) > 1 {
		b.client.SendChatAction(ctx, chatID, "upload_video")
		var mediaGroup []InputMedia
		for idx, item := range res.Items {
			if idx >= 10 {
				break
			}
			mType := "video"
			if item.Type == "image" {
				mType = "photo"
			}
			itemCaption := ""
			if idx == 0 {
				itemCaption = caption
			}
			mediaGroup = append(mediaGroup, InputMedia{
				Type:      mType,
				Media:     item.URL,
				Caption:   itemCaption,
				ParseMode: "HTML",
			})
		}

		err := b.client.SendMediaGroup(ctx, chatID, mediaGroup)
		if err == nil {
			return nil
		}
		log.Printf("[Bot] Direct URL SendMediaGroup failed (%v), falling back to item-by-item", err)
	}

	// Case 2: Single item (or carousel fallback)
	for _, item := range res.Items {
		itemType := item.Type
		if itemType == "image" {
			b.client.SendChatAction(ctx, chatID, "upload_photo")
			err := b.client.SendPhoto(ctx, chatID, item.URL, caption, nil)
			if err == nil {
				continue
			}
		} else {
			b.client.SendChatAction(ctx, chatID, "upload_video")
			err := b.client.SendVideo(ctx, chatID, item.URL, caption, nil)
			if err == nil {
				continue
			}
		}

		// Stream / Local Ephemeral Download & Upload Strategy
		downloaded, err := b.downloader.Download(ctx, item.URL, "media.mp4")
		if err != nil {
			return fmt.Errorf("stream download failed: %w", err)
		}
		defer downloaded.Close()

		uploadErr := b.client.SendLocalFile(ctx, chatID, downloaded.Path, itemType, caption, nil)
		if uploadErr != nil {
			return uploadErr
		}
	}

	return nil
}

func (b *Bot) forwardToLogChannel(ctx context.Context, userID int64, sourceURL string, res *resolver.MediaResult) error {
	if b.cfg.LogChannelID == "" || res == nil || len(res.Items) == 0 {
		return nil
	}

	channelChatID := parseChatID(b.cfg.LogChannelID)
	if channelChatID == 0 {
		log.Printf("[LogChannel] Invalid channel ID: %s", b.cfg.LogChannelID)
		return nil
	}

	logCaption := fmt.Sprintf("📥 <b>New Download</b>\n👤 <b>User:</b> <code>%d</code>\n🔗 <b>Source:</b> <a href=\"%s\">Instagram Link</a>", userID, sourceURL)

	// Case 1: Carousel / Gallery (Multiple Items)
	if len(res.Items) > 1 {
		var mediaGroup []InputMedia
		for idx, item := range res.Items {
			if idx >= 10 {
				break
			}
			mType := "video"
			if item.Type == "image" {
				mType = "photo"
			}
			itemCaption := ""
			if idx == 0 {
				itemCaption = logCaption
			}
			mediaGroup = append(mediaGroup, InputMedia{
				Type:      mType,
				Media:     item.URL,
				Caption:   itemCaption,
				ParseMode: "HTML",
			})
		}

		err := b.client.SendMediaGroup(ctx, channelChatID, mediaGroup)
		if err == nil {
			return nil
		}
		log.Printf("[LogChannel] Direct URL SendMediaGroup failed (%v), falling back to individual items", err)
	}

	// Case 2: Single item (or carousel fallback)
	for idx, item := range res.Items {
		itemCaption := ""
		if idx == 0 {
			itemCaption = logCaption
		}

		var sendErr error
		if item.Type == "image" {
			sendErr = b.client.SendPhoto(ctx, channelChatID, item.URL, itemCaption, nil)
		} else {
			sendErr = b.client.SendVideo(ctx, channelChatID, item.URL, itemCaption, nil)
		}

		if sendErr != nil {
			log.Printf("[LogChannel] Direct URL send failed for item %d (%v), trying local download", idx, sendErr)
			ext := ".mp4"
			if item.Type == "image" {
				ext = ".jpg"
			}
			downloaded, err := b.downloader.Download(ctx, item.URL, "log_media"+ext)
			if err == nil {
				defer downloaded.Close()
				_ = b.client.SendLocalFile(ctx, channelChatID, downloaded.Path, item.Type, itemCaption, nil)
			}
		}
	}

	return nil
}

func parseChatID(str string) int64 {
	var id int64
	_, _ = fmt.Sscanf(strings.TrimSpace(str), "%d", &id)
	return id
}

func (b *Bot) handleCallbackQuery(ctx context.Context, cb *CallbackQuery) error {
	userID := cb.From.ID
	var chatID int64
	var messageID int64
	if cb.Message != nil {
		chatID = cb.Message.Chat.ID
		messageID = cb.Message.MessageID
	}
	data := cb.Data

	_ = b.client.AnswerCallbackQuery(ctx, cb.ID, "")

	// 1. Carousel Slider Button Clicked (◀️ Prev / Next ▶️)
	if strings.HasPrefix(data, "slide:") {
		return b.handleCarouselSlide(ctx, cb, data)
	}

	// 2. Language update clicked: Edit SAME message in-place
	if strings.HasPrefix(data, "lang:") {
		newLang := strings.TrimPrefix(data, "lang:")
		i18n.GlobalUserLangStore.Set(userID, newLang)
		if b.db != nil {
			go func(uid int64, l string) {
				dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = b.db.SetUserLanguage(dbCtx, uid, l)
			}(userID, newLang)
		}
		t := i18n.ForUser(userID)
		if cb.Message != nil {
			return b.client.EditMessageText(ctx, chatID, messageID, t.LangUpdated+"\n\n"+t.StartMessage, b.getMainKeyboard(userID))
		}
		return nil
	}

	if data == "verify_forcesub" {
		if b.checkForceSub(ctx, userID) {
			if cb.Message != nil {
				_ = b.client.DeleteMessage(ctx, chatID, messageID)
			}
			_, err := b.client.SendMessage(ctx, chatID, "✅ <b>Verification Successful!</b>\nYou can now send any Instagram link to download.", b.getMainKeyboard(userID))
			return err
		}
		_ = b.client.AnswerCallbackQuery(ctx, cb.ID, "⚠️ You haven't joined the channel yet! Please join first.")
		return nil
	}

	t := i18n.ForUser(userID)
	switch data {
	case "action_help":
		return b.client.EditMessageText(ctx, chatID, messageID, b.getHelpMessage(ctx, userID), b.getHelpKeyboard())
	case "action_about":
		return b.client.EditMessageText(ctx, chatID, messageID, b.getAboutMessage(ctx, userID), b.getAboutKeyboard())
	case "action_lang":
		return b.editLanguageSelector(ctx, chatID, messageID, userID)
	case "action_start":
		return b.client.EditMessageText(ctx, chatID, messageID, t.StartMessage, b.getMainKeyboard(userID))
	}

	return nil
}

func (b *Bot) sendStartMessage(ctx context.Context, chatID, userID int64) error {
	t := i18n.ForUser(userID)
	_, err := b.client.SendMessage(ctx, chatID, t.StartMessage, b.getMainKeyboard(userID))
	return err
}

func (b *Bot) sendForceSubMessage(ctx context.Context, chatID, userID int64) error {
	t := i18n.ForUser(userID)
	channelLink := b.cfg.ForceSubChannelLink
	if channelLink == "" && strings.HasPrefix(b.cfg.ForceSubChannel, "@") {
		channelLink = "https://t.me/" + strings.TrimPrefix(b.cfg.ForceSubChannel, "@")
	}

	kb := &InlineKeyboardMarkup{
		InlineKeyboard: [][]InlineKeyboardButton{
			{
				{Text: t.BtnJoinChannel, URL: channelLink},
			},
			{
				{Text: t.BtnJoined, CallbackData: "verify_forcesub"},
			},
		},
	}
	_, err := b.client.SendMessage(ctx, chatID, t.ForceSubMsg, kb)
	return err
}

func (b *Bot) editLanguageSelector(ctx context.Context, chatID, messageID, userID int64) error {
	t := i18n.ForUser(userID)
	kb := b.buildLanguageKeyboard()
	return b.client.EditMessageText(ctx, chatID, messageID, t.SelectLang, kb)
}

func (b *Bot) sendLanguageSelectorInNewMessage(ctx context.Context, chatID, userID int64) error {
	t := i18n.ForUser(userID)
	kb := b.buildLanguageKeyboard()
	_, err := b.client.SendMessage(ctx, chatID, t.SelectLang, kb)
	return err
}

func (b *Bot) buildLanguageKeyboard() *InlineKeyboardMarkup {
	var rows [][]InlineKeyboardButton
	var currentRow []InlineKeyboardButton

	for _, l := range i18n.SupportedLanguages {
		btn := InlineKeyboardButton{
			Text:         fmt.Sprintf("%s %s", l.Flag, l.Name),
			CallbackData: "lang:" + l.Code,
		}
		currentRow = append(currentRow, btn)
		if len(currentRow) == 2 {
			rows = append(rows, currentRow)
			currentRow = nil
		}
	}
	if len(currentRow) > 0 {
		rows = append(rows, currentRow)
	}

	rows = append(rows, []InlineKeyboardButton{
		{Text: "🔙 Back", CallbackData: "action_start"},
	})

	return &InlineKeyboardMarkup{InlineKeyboard: rows}
}

func (b *Bot) getMainKeyboard(userID int64) *InlineKeyboardMarkup {
	t := i18n.ForUser(userID)
	return &InlineKeyboardMarkup{
		InlineKeyboard: [][]InlineKeyboardButton{
			{
				{Text: t.BtnHelp, CallbackData: "action_help"},
				{Text: t.BtnAbout, CallbackData: "action_about"},
			},
			{
				{Text: t.BtnLanguage, CallbackData: "action_lang"},
			},
		},
	}
}

func (b *Bot) getAboutMessage(ctx context.Context, userID int64) string {
	name, username := b.getBotInfo(ctx)
	_ = userID

	return fmt.Sprintf(
		"🤖 <b>About %s</b>\n\n"+
			"<blockquote>⚡ <b>Bot:</b> %s (@%s)\n"+
			"👨‍💻 <b>Developer:</b> <a href=\"github.com/mrabhi2k3\">MrAbhi2k3</a>\n"+
			"📢 <b>Channel:</b> @TeleRoidGroup\n"+
			"💬 <b>Support:</b> @TeleRoid14\n"+
			"🛠 <b>Language:</b> Golang</blockquote>\n\n"+
			"✨ <i>Ultra-fast, Instagram downloader Reels, Posts, and Albums in original HD quality!</i>",
		name, name, username,
	)
}

func (b *Bot) getHelpMessage(ctx context.Context, userID int64) string {
	_ = userID
	_, username := b.getBotInfo(ctx)

	return fmt.Sprintf(
		"📖 <b>How to Use @%s:</b>\n\n"+
			"1️⃣ <b>Direct Chat:</b>\n"+
			"• Copy any link from Instagram (Reel, Video, Photo, Album).\n"+
			"• Send the link directly here to get media instantly!\n\n"+
			"2️⃣ <b>Inline Mode (Any Chat/Group):</b>\n"+
			"• Type <code>@%s &lt;Instagram URL&gt;</code> in any chat.\n"+
			"• Tap the preview to send the media directly!\n\n"+
			"💬 <i>Need help or have suggestions? Join our support group below!</i>",
		username, username,
	)
}

func (b *Bot) getAboutKeyboard() *InlineKeyboardMarkup {
	return &InlineKeyboardMarkup{
		InlineKeyboard: [][]InlineKeyboardButton{
			{
				{Text: "📢 Updates Channel", URL: "https://t.me/moviesflixers_dl"},
				{Text: "👨‍💻 Developer", URL: "https://github.com/mrabhi2k3"},
			},

			{
				{Text: "🔙 Back to Menu", CallbackData: "action_start"},
			},
		},
	}
}

func (b *Bot) getHelpKeyboard() *InlineKeyboardMarkup {
	return &InlineKeyboardMarkup{
		InlineKeyboard: [][]InlineKeyboardButton{
			{
				{Text: "📢 Channel", URL: "https://t.me/teleroidgroup"},
				{Text: "💬 Support", URL: "https://t.me/teleroid14"},
			},
			{
				{Text: "🔙 Back to Menu", CallbackData: "action_start"},
			},
		},
	}
}

func (b *Bot) getBackKeyboard() *InlineKeyboardMarkup {
	return &InlineKeyboardMarkup{
		InlineKeyboard: [][]InlineKeyboardButton{
			{
				{Text: "🔙 Back to Menu", CallbackData: "action_start"},
			},
		},
	}
}

// handleCarouselSlide flips through carousel items directly inside the chat using Prev / Next
func (b *Bot) handleCarouselSlide(ctx context.Context, cb *CallbackQuery, data string) error {
	parts := strings.Split(data, ":")
	if len(parts) < 3 {
		return nil
	}

	shortcode := parts[1]
	var targetIdx int
	_, _ = fmt.Sscanf(parts[2], "%d", &targetIdx)

	reconstructURL := "https://www.instagram.com/p/" + shortcode + "/"
	mediaRes, err := b.resolver.Resolve(ctx, reconstructURL)
	if err != nil || mediaRes == nil || len(mediaRes.Items) == 0 {
		return nil
	}

	total := len(mediaRes.Items)
	if targetIdx < 0 || targetIdx >= total {
		targetIdx = 0
	}

	item := mediaRes.Items[targetIdx]
	mType := "photo"
	if item.Type == "video" {
		mType = "video"
	}

	caption := fmt.Sprintf("✨ Downloaded via %s", b.getBotUsername(ctx))
	inputMedia := InputMedia{
		Type:      mType,
		Media:     item.URL,
		Caption:   caption,
		ParseMode: "HTML",
	}

	prevIdx := (targetIdx - 1 + total) % total
	nextIdx := (targetIdx + 1) % total

	sliderKB := &InlineKeyboardMarkup{
		InlineKeyboard: [][]InlineKeyboardButton{
			{
				{Text: "◀️ Prev", CallbackData: fmt.Sprintf("slide:%s:%d", shortcode, prevIdx)},
				{Text: fmt.Sprintf("%d/%d", targetIdx+1, total), CallbackData: "noop"},
				{Text: "Next ▶️", CallbackData: fmt.Sprintf("slide:%s:%d", shortcode, nextIdx)},
			},
			{
				{Text: "🔗 Instagram Link", URL: reconstructURL},
			},
		},
	}

	if cb.InlineMessageID != "" {
		return b.client.EditInlineMessageMedia(ctx, cb.InlineMessageID, inputMedia, sliderKB)
	} else if cb.Message != nil {
		return b.client.EditMessageMedia(ctx, cb.Message.Chat.ID, cb.Message.MessageID, inputMedia, sliderKB)
	}

	return nil
}

// handleStatsCommand handles /stats for bot owners.
func (b *Bot) handleStatsCommand(ctx context.Context, chatID, userID int64) error {
	if !b.cfg.IsOwner(userID) {
		_, err := b.client.SendMessage(ctx, chatID, "⛔ <b>Access Denied:</b> This command is restricted to the bot owner.", nil)
		return err
	}

	if b.db == nil {
		_, err := b.client.SendMessage(ctx, chatID, "⚠️ <b>MongoDB is not configured or connected.</b>\nPlease set <code>MONGODB_URI</code> in environment variables.", nil)
		return err
	}

	stats, err := b.db.GetStats(ctx)
	if err != nil {
		_, err := b.client.SendMessage(ctx, chatID, fmt.Sprintf("❌ Error retrieving database stats: %v", err), nil)
		return err
	}

	msg := fmt.Sprintf(`📊 <b>Bot Database Statistics</b>

👥 <b>Total Users:</b> <code>%d</code>
🟢 <b>Active (Last 24h):</b> <code>%d</code>
🚫 <b>Blocked Users:</b> <code>%d</code>
⛔ <b>Banned Users:</b> <code>%d</code>
💬 <b>Total Messages Logged:</b> <code>%d</code>
📥 <b>Total Downloads Processed:</b> <code>%d</code>

<i>Database: %s</i>`,
		stats.TotalUsers,
		stats.Active24hUsers,
		stats.BlockedUsers,
		stats.BannedUsers,
		stats.TotalMessages,
		stats.TotalDownloads,
		b.cfg.MongoDBName,
	)

	_, err = b.client.SendMessage(ctx, chatID, msg, nil)
	return err
}

func (b *Bot) handleBanCommand(ctx context.Context, chatID, userID int64, text string) error {
	if !b.cfg.IsOwner(userID) {
		_, err := b.client.SendMessage(ctx, chatID, "⛔ <b>Access Denied:</b> This command is restricted to the bot owner.", nil)
		return err
	}

	if b.db == nil {
		_, err := b.client.SendMessage(ctx, chatID, "⚠️ <b>MongoDB is not configured.</b>", nil)
		return err
	}

	parts := strings.Fields(text)
	if len(parts) < 2 {
		_, err := b.client.SendMessage(ctx, chatID, "Usage: <code>/ban &lt;user_id&gt; [reason]</code>", nil)
		return err
	}

	targetID := parseChatID(parts[1])
	if targetID == 0 {
		_, err := b.client.SendMessage(ctx, chatID, "❌ Invalid user ID.", nil)
		return err
	}

	reason := ""
	if len(parts) > 2 {
		reason = strings.Join(parts[2:], " ")
	}

	err := b.db.BanUser(ctx, targetID, reason)
	if err != nil {
		_, err := b.client.SendMessage(ctx, chatID, fmt.Sprintf("❌ Error banning user: %v", err), nil)
		return err
	}

	_, err = b.client.SendMessage(ctx, chatID, fmt.Sprintf("✅ User <code>%d</code> has been banned.", targetID), nil)
	return err
}

func (b *Bot) handleUnbanCommand(ctx context.Context, chatID, userID int64, text string) error {
	if !b.cfg.IsOwner(userID) {
		_, err := b.client.SendMessage(ctx, chatID, "⛔ <b>Access Denied:</b> This command is restricted to the bot owner.", nil)
		return err
	}

	if b.db == nil {
		_, err := b.client.SendMessage(ctx, chatID, "⚠️ <b>MongoDB is not configured.</b>", nil)
		return err
	}

	parts := strings.Fields(text)
	if len(parts) < 2 {
		_, err := b.client.SendMessage(ctx, chatID, "Usage: <code>/unban &lt;user_id&gt;</code>", nil)
		return err
	}

	targetID := parseChatID(parts[1])
	if targetID == 0 {
		_, err := b.client.SendMessage(ctx, chatID, "❌ Invalid user ID.", nil)
		return err
	}

	err := b.db.UnbanUser(ctx, targetID)
	if err != nil {
		_, err := b.client.SendMessage(ctx, chatID, fmt.Sprintf("❌ Error unbanning user: %v", err), nil)
		return err
	}

	_, err = b.client.SendMessage(ctx, chatID, fmt.Sprintf("✅ User <code>%d</code> has been unbanned.", targetID), nil)
	return err
}

// handleBroadcastCommand broadcasts a message to all registered users.
// Supports both:
// 1. /broadcast <text message>
// 2. Replying to any message (media, video, photo, document, sticker) with /broadcast
func (b *Bot) handleBroadcastCommand(ctx context.Context, msg *Message) error {
	chatID := msg.Chat.ID
	userID := msg.From.ID

	if !b.cfg.IsOwner(userID) {
		_, err := b.client.SendMessage(ctx, chatID, "⛔ <b>Access Denied:</b> This command is restricted to the bot owner.", nil)
		return err
	}

	if b.db == nil {
		_, err := b.client.SendMessage(ctx, chatID, "⚠️ <b>MongoDB is not configured or connected.</b>\nPlease set <code>MONGODB_URI</code> in environment variables.", nil)
		return err
	}

	text := strings.TrimSpace(msg.Text)
	broadcastText := strings.TrimSpace(strings.TrimPrefix(text, "/broadcast"))

	isReply := msg.ReplyToMessage != nil
	if !isReply && broadcastText == "" {
		helpMsg := `📢 <b>Broadcast Usage:</b>

1. <b>Send Text Broadcast:</b>
<code>/broadcast Hello everyone! Here is an update...</code>

2. <b>Send Media/Forward Broadcast:</b>
Send or forward any photo, video, audio, sticker or file to the bot, and <b>reply</b> to it with <code>/broadcast</code>.`
		_, err := b.client.SendMessage(ctx, chatID, helpMsg, nil)
		return err
	}

	userIDs, err := b.db.GetAllUserIDs(ctx, false)
	if err != nil {
		_, err := b.client.SendMessage(ctx, chatID, fmt.Sprintf("❌ Failed to fetch user IDs: %v", err), nil)
		return err
	}

	total := len(userIDs)
	if total == 0 {
		_, err := b.client.SendMessage(ctx, chatID, "⚠️ No users found in database to broadcast to.", nil)
		return err
	}

	progressMsgID, _ := b.client.SendMessage(ctx, chatID, fmt.Sprintf("🚀 <b>Broadcasting started...</b>\nTarget recipients: <b>%d</b> users", total), nil)

	// Launch broadcast execution in background goroutine with independent context
	go func(targetIDs []int64, repMsg *Message, bText string, replyMsgID int64, ownerChatID, progID int64) {
		bCtx := context.Background()
		var successCount, failedCount, blockedCount int

		for i, targetID := range targetIDs {
			var sendErr error
			if isReply && repMsg != nil {
				_, sendErr = b.client.CopyMessage(bCtx, targetID, ownerChatID, repMsg.MessageID)
			} else {
				_, sendErr = b.client.SendMessage(bCtx, targetID, bText, nil)
			}

			if sendErr != nil {
				errStr := sendErr.Error()
				if strings.Contains(errStr, "403") || strings.Contains(errStr, "blocked") || strings.Contains(errStr, "user is deactivated") {
					blockedCount++
					_ = b.db.MarkUserBlocked(bCtx, targetID)
				} else {
					failedCount++
				}
			} else {
				successCount++
			}

			// Telegram Bot API allows ~30 messages per second.
			// 35ms sleep = ~28 messages per second to safely avoid Telegram rate limits.
			time.Sleep(35 * time.Millisecond)

			// Progress update every 100 users
			if (i+1)%100 == 0 && progID > 0 {
				_ = b.client.EditMessageText(bCtx, ownerChatID, progID,
					fmt.Sprintf("🚀 <b>Broadcasting in progress...</b>\nProgress: <code>%d/%d</code>\n✅ Sent: <code>%d</code> | 🚫 Blocked: <code>%d</code> | ❌ Failed: <code>%d</code>",
						i+1, total, successCount, blockedCount, failedCount), nil)
			}
		}

		// Final report to Owner
		summary := fmt.Sprintf(`✅ <b>Broadcast Completed!</b>

👥 <b>Total Recipients:</b> <code>%d</code>
✅ <b>Delivered:</b> <code>%d</code>
🚫 <b>Blocked/Deactivated:</b> <code>%d</code>
❌ <b>Other Failures:</b> <code>%d</code>`,
			total, successCount, blockedCount, failedCount)

		if b.db != nil {
			_ = b.db.RecordBroadcast(bCtx, db.BroadcastLog{
				BroadcastID: fmt.Sprintf("bc_%d", time.Now().Unix()),
				OwnerID:     ownerChatID,
				TotalUsers:  total,
				Delivered:   successCount,
				Blocked:     blockedCount,
				Failed:      failedCount,
				StartedAt:   time.Now(),
				CompletedAt: time.Now(),
			})
		}

		if progID > 0 {
			_ = b.client.EditMessageText(bCtx, ownerChatID, progID, summary, nil)
		} else {
			_, _ = b.client.SendMessage(bCtx, ownerChatID, summary, nil)
		}
	}(userIDs, msg.ReplyToMessage, broadcastText, msg.MessageID, chatID, progressMsgID)

	return nil
}
