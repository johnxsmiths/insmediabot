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

// handleInlineQuery processes inline queries (e.g. @botusername https://instagram.com/reel/...).
func (b *Bot) handleInlineQuery(ctx context.Context, iq *InlineQuery) error {
	if iq == nil {
		return nil
	}

	userID := iq.From.ID
	query := strings.TrimSpace(iq.Query)
	t := i18n.ForUser(userID)

	// ForceSub Check in inline mode
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

	// If query is empty or not an Instagram link, offer helpful guidance with Instagram Logo
	if query == "" || !resolver.IsInstagramURL(query) {
		article := InlineQueryResultArticle{
			Type:         "article",
			ID:           "help_guide",
			Title:        "📥 Paste Instagram Link here",
			Description:  "Type: @" + strings.TrimPrefix(b.getBotUsername(ctx), "@") + " <Instagram URL>",
			ThumbnailURL: igLogoURL,
			InputMessageContent: InputTextMessageContent{
				MessageText: t.StartMessage,
				ParseMode:   "HTML",
			},
			ReplyMarkup: b.getMainKeyboard(userID),
		}
		return b.client.AnswerInlineQuery(ctx, iq.ID, []interface{}{article}, 5)
	}

	// Normalize Instagram URL
	cleanURL, err := resolver.NormalizeInstagramURL(query)
	if err != nil {
		cleanURL = query
	}

	// Resolve the Instagram media via our fallback chain
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

	// Mirror inline resolution to Log Channel in background if configured
	if b.cfg.LogChannelID != "" {
		go func() {
			logCtx, logCancel := context.WithTimeout(context.Background(), 40*time.Second)
			defer logCancel()
			_ = b.forwardToLogChannel(logCtx, userID, cleanURL, mediaRes)
		}()
	}

	// Build inline media results from resolved items
	var results []interface{}
	caption := fmt.Sprintf("✨ Downloaded via %s", b.getBotUsername(ctx))

	for idx, item := range mediaRes.Items {
		if idx >= 10 {
			break
		}

		itemTitle := fmt.Sprintf("Instagram %s %d", strings.Title(item.Type), idx+1)

		if item.Type == "image" {
			results = append(results, InlineQueryResultPhoto{
				Type:         "photo",
				ID:           fmt.Sprintf("photo_%d", idx),
				PhotoURL:     item.URL,
				ThumbnailURL: item.URL,
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
				Type:      "video",
				ID:        fmt.Sprintf("video_%d", idx),
				VideoURL:  item.URL,
				MimeType:  "video/mp4",
				Title:     itemTitle,
				Caption:   caption,
				ParseMode: "HTML",
				ReplyMarkup: &InlineKeyboardMarkup{
					InlineKeyboard: [][]InlineKeyboardButton{
						{{Text: "🔗 Instagram Link", URL: cleanURL}},
					},
				},
			})
		}
	}

	return b.client.AnswerInlineQuery(ctx, iq.ID, results, 60)
}
