package telegram

import (
	"context"
	"fmt"
	"strings"

	"slmedia/pkg/i18n"
	"slmedia/pkg/resolver"
)

// handleInlineQuery processes inline queries (e.g. @botusername https://instagram.com/reel/...).
func (b *Bot) handleInlineQuery(ctx context.Context, iq *InlineQuery) error {
	if iq == nil {
		return nil
	}

	userID := iq.From.ID
	query := strings.TrimSpace(iq.Query)
	t := i18n.ForUser(userID)

	// If query is empty or not an Instagram link, offer helpful guidance
	if query == "" || !resolver.IsInstagramURL(query) {
		article := InlineQueryResultArticle{
			Type: "article",
			ID:   "help_guide",
			Title: "📥 Paste any Instagram Link here",
			Description: "Type: @" + strings.TrimPrefix(b.getBotUsername(ctx), "@") + " <Instagram URL>",
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
			Type:        "article",
			ID:          "error_not_found",
			Title:       "❌ Media Not Found",
			Description: "Could not resolve media from this Instagram link.",
			InputMessageContent: InputTextMessageContent{
				MessageText: fmt.Sprintf("%s\n\n🔗 <a href=\"%s\">Open Instagram Link</a>", t.DownloadError, cleanURL),
				ParseMode:   "HTML",
			},
		}
		return b.client.AnswerInlineQuery(ctx, iq.ID, []interface{}{article}, 10)
	}

	// Build inline media results from resolved items
	var results []interface{}
	caption := fmt.Sprintf("✨ Downloaded via %s", b.getBotUsername(ctx))

	for idx, item := range mediaRes.Items {
		// Limit inline results to max 10
		if idx >= 10 {
			break
		}

		itemTitle := fmt.Sprintf("Instagram %s %d", strings.Title(item.Type), idx+1)
		thumb := item.URL
		if item.Type == "video" {
			thumb = "https://cdn-icons-png.flaticon.com/512/174/174855.png" // Clean Instagram icon thumb fallback
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
						{{Text: "🔗 Direct Link", URL: cleanURL}},
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
						{{Text: "🔗 Direct Link", URL: cleanURL}},
					},
				},
			})
		}
	}

	return b.client.AnswerInlineQuery(ctx, iq.ID, results, 60)
}
