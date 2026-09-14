package telegram

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"slmedia/pkg/config"
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

	return &Bot{
		cfg:         cfg,
		client:      client,
		resolver:    resManager,
		storage:     tmpStorage,
		downloader:  downloader,
		rateLimiter: limiter,
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

	// 1. Handle Inline Keyboard Button Callbacks
	if update.CallbackQuery != nil {
		return b.handleCallbackQuery(ctx, update.CallbackQuery)
	}

	// 2. Handle Inline Mode Queries (@botusername <url>)
	if update.InlineQuery != nil {
		return b.handleInlineQuery(ctx, update.InlineQuery)
	}

	// 3. Handle Text Messages
	if update.Message != nil && update.Message.Text != "" {
		return b.handleTextMessage(ctx, update.Message)
	}

	return nil
}

func (b *Bot) handleTextMessage(ctx context.Context, msg *Message) error {
	chatID := msg.Chat.ID
	userID := msg.From.ID
	text := strings.TrimSpace(msg.Text)

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
		fallbackText := fmt.Sprintf("%s\n\n🔗 %s", t.UploadFallback, cleanURL)
		directKB := &InlineKeyboardMarkup{
			InlineKeyboard: [][]InlineKeyboardButton{
				{{Text: t.BtnDirectLink, URL: cleanURL}},
			},
		}
		_ = b.client.EditMessageText(ctx, chatID, statusMsgID, fallbackText, directKB)
		return nil
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

	logCaption := fmt.Sprintf("📥 <b>New Download</b>\n👤 <b>User:</b> <code>%d</code>\n🔗 <b>Source:</b> <a href=\"%s\">Instagram Link</a>", userID, sourceURL)

	firstItem := res.Items[0]
	if firstItem.Type == "image" {
		return b.client.SendPhoto(ctx, parseChatID(b.cfg.LogChannelID), firstItem.URL, logCaption, nil)
	}
	return b.client.SendVideo(ctx, parseChatID(b.cfg.LogChannelID), firstItem.URL, logCaption, nil)
}

func parseChatID(str string) int64 {
	var id int64
	_, _ = fmt.Sscanf(str, "%d", &id)
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
	// Format: slide:<shortcode>:<targetIdx>
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
