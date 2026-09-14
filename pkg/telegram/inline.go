package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	"slmedia/pkg/i18n"
	"slmedia/pkg/resolver"
)

const igLogoURL = "https://cdn-icons-png.flaticon.com/512/174/174855.png"

// handleInlineQuery resolves and returns direct media items (videos, photos, carousel items).
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
			Description:  "You must join our channel to use inline download.",
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

	// 2. If query is empty or not an Instagram link, offer helpful guidance with Instagram Logo
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

	// 3. Normalize Instagram URL
	cleanURL, err := resolver.NormalizeInstagramURL(query)
	if err != nil {
		cleanURL = query
	}

	// 4. Resolve the Instagram media via our fallback chain
	mediaRes, err := b.resolver.Resolve(ctx, cleanURL)
	if err != nil || mediaRes == nil || len(mediaRes.Items) == 0 {
		article := InlineQueryResultArticle{
			Type:         "article",
			ID:           "error_not_found",
			Title:        "❌ Media Not Found",
			Description:  "Could not resolve media from this Instagram link.",
			ThumbnailURL: igLogoURL,
			InputMessageContent: InputTextMessageContent{
				MessageText: fmt.Sprintf("%s\n\n🔗 <a href=\"%s\">Open Instagram Link</a>", t.DownloadError, cleanURL),
				ParseMode:   "HTML",
			},
		}
		return b.client.AnswerInlineQuery(ctx, iq.ID, []interface{}{article}, 10)
	}

	// 5. Mirror to Log Channel in background if configured
	if b.cfg.LogChannelID != "" {
		go func() {
			logCtx, logCancel := context.WithTimeout(context.Background(), 40*time.Second)
			defer logCancel()
			_ = b.forwardToLogChannel(logCtx, userID, cleanURL, mediaRes)
		}()
	}

	// 6. Build direct media results
	var results []interface{}
	caption := fmt.Sprintf("✨ Downloaded via %s", b.getBotUsername(ctx))

	if len(mediaRes.Items) > 1 {
		shortcode := resolver.ExtractShortcode(cleanURL)
		deepLinkURL := fmt.Sprintf("https://t.me/%s?start=dl_%s", botUser, shortcode)

		albumCard := InlineQueryResultArticle{
			Type:         "article",
			ID:           "album_all_" + shortcode,
			Title:        fmt.Sprintf("📦 Send Full Album / Carousel (%d items)", len(mediaRes.Items)),
			Description:  "Tap to get all images & videos delivered together as a full gallery",
			ThumbnailURL: igLogoURL,
			InputMessageContent: InputTextMessageContent{
				MessageText: fmt.Sprintf(
					"📸 <b>Instagram Carousel Album</b>\n\n"+
						"<blockquote>📦 <b>Total Items:</b> %d\n"+
						"🔗 <b>Source:</b> <a href=\"%s\">Instagram Post</a></blockquote>\n\n"+
						"👉 <i>Click the button below to get all %d items delivered as a full album in your chat!</i>",
					len(mediaRes.Items), cleanURL, len(mediaRes.Items),
				),
				ParseMode: "HTML",
			},
			ReplyMarkup: &InlineKeyboardMarkup{
				InlineKeyboard: [][]InlineKeyboardButton{
					{
						{Text: fmt.Sprintf("🚀 Deliver Full Album (%d items)", len(mediaRes.Items)), URL: deepLinkURL},
					},
					{
						{Text: "🔗 Instagram Link", URL: cleanURL},
					},
				},
			},
		}
		results = append(results, albumCard)
	}

	for idx, item := range mediaRes.Items {
		if idx >= 10 {
			break
		}

		itemTitle := fmt.Sprintf("Instagram %s %d", strings.Title(item.Type), idx+1)
		thumb := item.ThumbnailURL
		if thumb == "" {
			if item.Type == "image" {
				thumb = item.URL
			} else {
				thumb = igLogoURL // Fallback required by Telegram Bot API for videos
			}
		}

		if item.Type == "image" {
			results = append(results, InlineQueryResultPhoto{
				Type:         "photo",
				ID:           fmt.Sprintf("photo_%d", idx),
				PhotoURL:     item.URL,
				ThumbnailURL: thumb,
				Title:        itemTitle,
				Caption:      caption,
				ParseMode:    "HTML",
				ReplyMarkup: &InlineKeyboardMarkup{
					InlineKeyboard: [][]InlineKeyboardButton{
						{{Text: "🔗 Instagram Link", URL: cleanURL}},
					},
				},
			})
		} else {
			results = append(results, InlineQueryResultVideo{
				Type:         "video",
				ID:           fmt.Sprintf("video_%d", idx),
				VideoURL:     item.URL,
				MimeType:     "video/mp4",
				ThumbnailURL: thumb,
				Title:        itemTitle,
				Caption:      caption,
				ParseMode:    "HTML",
				ReplyMarkup: &InlineKeyboardMarkup{
					InlineKeyboard: [][]InlineKeyboardButton{
						{{Text: "🔗 Instagram Link", URL: cleanURL}},
					},
				},
			})
		}
	}

	return b.client.AnswerInlineQuery(ctx, iq.ID, results, 300)
}
