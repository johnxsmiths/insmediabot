package telegram

import (
	"context"
	"fmt"
	"strings"

	"slmedia/pkg/i18n"
	"slmedia/pkg/resolver"
)

const igLogoURL = "https://cdn-icons-png.flaticon.com/512/174/174855.png"

// handleInlineQuery processes inline queries instantly with zero lag.
// Users can tap the instant card, which delivers media into the chat with auto-download!
func (b *Bot) handleInlineQuery(ctx context.Context, iq *InlineQuery) error {
	if iq == nil {
		return nil
	}

	userID := iq.From.ID
	query := strings.TrimSpace(iq.Query)
	t := i18n.ForUser(userID)
	botUser := strings.TrimPrefix(b.getBotUsername(ctx), "@")

	// 1. ForceSub Check in inline mode
	if !b.checkForceSub(ctx, userID) {
		channelLink := b.cfg.ForceSubChannelLink
		if channelLink == "" && strings.HasPrefix(b.cfg.ForceSubChannel, "@") {
			channelLink = "https://t.me/" + strings.TrimPrefix(b.cfg.ForceSubChannel, "@")
		}

		article := InlineQueryResultArticle{
			Type:         "article",
			ID:           "forcesub_required",
			Title:        "📢 Channel Subscription Required",
			Description:  "Join our channel to unlock inline downloading.",
			ThumbnailURL: igLogoURL,
			InputMessageContent: InputTextMessageContent{
				MessageText: t.ForceSubMsg,
				ParseMode:   "HTML",
			},
			ReplyMarkup: &InlineKeyboardMarkup{
				InlineKeyboard: [][]InlineKeyboardButton{
					{{Text: t.BtnJoinChannel, URL: channelLink}},
				},
			},
		}
		return b.client.AnswerInlineQuery(ctx, iq.ID, []interface{}{article}, 1)
	}

	// 2. If query is empty or not an Instagram link, show instant guidance card
	if query == "" || !resolver.IsInstagramURL(query) {
		article := InlineQueryResultArticle{
			Type:         "article",
			ID:           "help_guide",
			Title:        "📥 Paste Instagram Link here",
			Description:  "Type: @" + botUser + " <Instagram URL>",
			ThumbnailURL: igLogoURL,
			InputMessageContent: InputTextMessageContent{
				MessageText: t.StartMessage,
				ParseMode:   "HTML",
			},
			ReplyMarkup: b.getMainKeyboard(userID),
		}
		return b.client.AnswerInlineQuery(ctx, iq.ID, []interface{}{article}, 5)
	}

	// 3. Instant Instantaneous Inline Card (0ms delay - no waiting for downloads!)
	cleanURL, err := resolver.NormalizeInstagramURL(query)
	if err != nil {
		cleanURL = query
	}

	shortcode := resolver.ExtractShortcode(cleanURL)
	if shortcode == "" {
		shortcode = "Media"
	}

	// Instant Article: Clicking this immediately sends the processing card into the chat
	article := InlineQueryResultArticle{
		Type:         "article",
		ID:           "dl_" + shortcode,
		Title:        fmt.Sprintf("⚡ Download Instagram (%s)", shortcode),
		Description:  "Tap to fetch and deliver this Reel/Post/Album",
		ThumbnailURL: igLogoURL,
		InputMessageContent: InputTextMessageContent{
			MessageText: fmt.Sprintf("🔎 <b>Processing Instagram link...</b>\n\n🔗 <code>%s</code>\n\n⚡ <i>Fetching media, please wait...</i>", cleanURL),
			ParseMode:   "HTML",
		},
		ReplyMarkup: &InlineKeyboardMarkup{
			InlineKeyboard: [][]InlineKeyboardButton{
				{
					{Text: "⬇️ Fetching Media...", CallbackData: "inlinedl:" + cleanURL},
				},
			},
		},
	}

	return b.client.AnswerInlineQuery(ctx, iq.ID, []interface{}{article}, 300)
}
