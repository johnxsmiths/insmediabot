package telegram

import "encoding/json"

// Update represents an incoming Telegram webhook update payload.
type Update struct {
	UpdateID      int64          `json:"update_id"`
	Message       *Message       `json:"message,omitempty"`
	CallbackQuery *CallbackQuery `json:"callback_query,omitempty"`
	InlineQuery   *InlineQuery   `json:"inline_query,omitempty"`
}

// InlineQuery represents an incoming inline query (e.g. @botname <url>).
type InlineQuery struct {
	ID       string `json:"id"`
	From     User   `json:"from"`
	Query    string `json:"query"`
	Offset   string `json:"offset"`
	ChatType string `json:"chat_type,omitempty"`
}

// InlineQueryResultVideo represents a video result in an inline query.
type InlineQueryResultVideo struct {
	Type          string                `json:"type"` // "video"
	ID            string                `json:"id"`
	VideoURL      string                `json:"video_url"`
	MimeType      string                `json:"mime_type"`
	ThumbnailURL  string                `json:"thumbnail_url,omitempty"`
	Title         string                `json:"title"`
	Caption       string                `json:"caption,omitempty"`
	ParseMode     string                `json:"parse_mode,omitempty"`
	ReplyMarkup   *InlineKeyboardMarkup `json:"reply_markup,omitempty"`
}

// InlineQueryResultPhoto represents a photo result in an inline query.
type InlineQueryResultPhoto struct {
	Type         string                `json:"type"` // "photo"
	ID           string                `json:"id"`
	PhotoURL     string                `json:"photo_url"`
	ThumbnailURL string                `json:"thumbnail_url,omitempty"`
	Title        string                `json:"title,omitempty"`
	Caption      string                `json:"caption,omitempty"`
	ParseMode    string                `json:"parse_mode,omitempty"`
	ReplyMarkup  *InlineKeyboardMarkup `json:"reply_markup,omitempty"`
}

// InlineQueryResultArticle represents an informative article result in an inline query.
type InlineQueryResultArticle struct {
	Type                string                `json:"type"` // "article"
	ID                  string                `json:"id"`
	Title               string                `json:"title"`
	InputMessageContent InputTextMessageContent `json:"input_message_content"`
	ReplyMarkup         *InlineKeyboardMarkup `json:"reply_markup,omitempty"`
	Description         string                `json:"description,omitempty"`
	ThumbnailURL        string                `json:"thumbnail_url,omitempty"`
}

// InputTextMessageContent represents the content of a text message to be sent as the result of an inline query.
type InputTextMessageContent struct {
	MessageText string `json:"message_text"`
	ParseMode   string `json:"parse_mode,omitempty"`
}

// User represents a Telegram user.
type User struct {
	ID           int64  `json:"id"`
	IsBot        bool   `json:"is_bot"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name,omitempty"`
	Username     string `json:"username,omitempty"`
	LanguageCode string `json:"language_code,omitempty"`
}

// Chat represents a Telegram chat.
type Chat struct {
	ID       int64  `json:"id"`
	Type     string `json:"type"`
	Title    string `json:"title,omitempty"`
	Username string `json:"username,omitempty"`
}

// Message represents a message.
type Message struct {
	MessageID      int64    `json:"message_id"`
	From           *User    `json:"from,omitempty"`
	Chat           Chat     `json:"chat"`
	Date           int64    `json:"date"`
	Text           string   `json:"text,omitempty"`
	Caption        string   `json:"caption,omitempty"`
	ReplyToMessage *Message `json:"reply_to_message,omitempty"`
}

// CallbackQuery represents an incoming callback query from an inline keyboard button.
type CallbackQuery struct {
	ID              string   `json:"id"`
	From            User     `json:"from"`
	Message         *Message `json:"message,omitempty"`
	InlineMessageID string   `json:"inline_message_id,omitempty"`
	Data            string   `json:"data"`
}

// InlineKeyboardMarkup represents inline keyboard with rows of buttons.
type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}

// InlineKeyboardButton represents one button in an inline keyboard.
type InlineKeyboardButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data,omitempty"`
	URL          string `json:"url,omitempty"`
}

// InputMedia represents an item in a media group (carousel).
type InputMedia struct {
	Type      string `json:"type"` // "photo" or "video"
	Media     string `json:"media"`
	Caption   string `json:"caption,omitempty"`
	ParseMode string `json:"parse_mode,omitempty"`
}

// APIResponse is the standard response from Telegram Bot API calls.
type APIResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result,omitempty"`
	ErrorCode   int             `json:"error_code,omitempty"`
	Description string          `json:"description,omitempty"`
}
