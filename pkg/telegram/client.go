package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Client wraps all direct Telegram Bot API HTTP calls.
type Client struct {
	token      string
	baseURL    string
	httpClient *http.Client
}

// NewClient returns a new Telegram Bot API client.
func NewClient(token string) *Client {
	return &Client{
		token:      token,
		baseURL:    "https://api.telegram.org/bot" + token,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// SendMessage sends a text message with optional inline keyboard.
func (c *Client) SendMessage(ctx context.Context, chatID int64, text string, replyMarkup *InlineKeyboardMarkup) (int64, error) {
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "HTML",
	}
	if replyMarkup != nil {
		payload["reply_markup"] = replyMarkup
	}

	res, err := c.postJSON(ctx, "/sendMessage", payload)
	if err != nil {
		return 0, err
	}
	var msgResult struct {
		MessageID int64 `json:"message_id"`
	}
	_ = json.Unmarshal(res.Result, &msgResult)
	return msgResult.MessageID, nil
}

// EditMessageText edits existing message text.
func (c *Client) EditMessageText(ctx context.Context, chatID int64, messageID int64, text string, replyMarkup *InlineKeyboardMarkup) error {
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"message_id": messageID,
		"text":       text,
		"parse_mode": "HTML",
	}
	if replyMarkup != nil {
		payload["reply_markup"] = replyMarkup
	}

	_, err := c.postJSON(ctx, "/editMessageText", payload)
	if err != nil && strings.Contains(err.Error(), "message is not modified") {
		return nil
	}
	return err
}

// EditInlineMessageText edits an inline message text sent via inline query.
func (c *Client) EditInlineMessageText(ctx context.Context, inlineMessageID string, text string, replyMarkup *InlineKeyboardMarkup) error {
	payload := map[string]interface{}{
		"inline_message_id": inlineMessageID,
		"text":              text,
		"parse_mode":        "HTML",
	}
	if replyMarkup != nil {
		payload["reply_markup"] = replyMarkup
	}

	_, err := c.postJSON(ctx, "/editMessageText", payload)
	if err != nil && strings.Contains(err.Error(), "message is not modified") {
		return nil
	}
	return err
}

// GetMe returns current bot user profile (to dynamically fetch bot username).
func (c *Client) GetMe(ctx context.Context) (*User, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/getMe", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apiRes struct {
		OK     bool `json:"ok"`
		Result User `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiRes); err != nil {
		return nil, err
	}
	if !apiRes.OK {
		return nil, fmt.Errorf("failed getMe")
	}
	return &apiRes.Result, nil
}

// DeleteMessage deletes a message by chatID and messageID.
func (c *Client) DeleteMessage(ctx context.Context, chatID int64, messageID int64) error {
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"message_id": messageID,
	}
	_, err := c.postJSON(ctx, "/deleteMessage", payload)
	return err
}

// GetChatMember checks if a user is a member of the given channel/chat.
func (c *Client) GetChatMember(ctx context.Context, channelID string, userID int64) (string, error) {
	payload := map[string]interface{}{
		"chat_id": channelID,
		"user_id": userID,
	}
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/getChatMember", bytes.NewReader(jsonBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var memberResp struct {
		OK     bool `json:"ok"`
		Result struct {
			Status string `json:"status"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&memberResp); err != nil {
		return "", err
	}
	if !memberResp.OK {
		return "", fmt.Errorf("failed getChatMember")
	}
	return memberResp.Result.Status, nil
}

// AnswerCallbackQuery acknowledges a callback query from an inline button.
func (c *Client) AnswerCallbackQuery(ctx context.Context, callbackID string, text string) error {
	payload := map[string]interface{}{
		"callback_query_id": callbackID,
	}
	if text != "" {
		payload["text"] = text
	}
	_, err := c.postJSON(ctx, "/answerCallbackQuery", payload)
	return err
}

// AnswerInlineQuery sends answers to an inline query.
func (c *Client) AnswerInlineQuery(ctx context.Context, inlineQueryID string, results []interface{}, cacheTime int) error {
	payload := map[string]interface{}{
		"inline_query_id": inlineQueryID,
		"results":         results,
		"cache_time":      cacheTime,
		"is_personal":     true,
	}
	_, err := c.postJSON(ctx, "/answerInlineQuery", payload)
	return err
}

// SendChatAction sends status indicator (typing, upload_video, upload_photo).
func (c *Client) SendChatAction(ctx context.Context, chatID int64, action string) {
	payload := map[string]interface{}{
		"chat_id": chatID,
		"action":  action,
	}
	_, _ = c.postJSON(ctx, "/sendChatAction", payload)
}

// SendVideo sends a video to the chat via direct URL.
func (c *Client) SendVideo(ctx context.Context, chatID int64, videoURL string, caption string, replyMarkup *InlineKeyboardMarkup) error {
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"video":      videoURL,
		"caption":    caption,
		"parse_mode": "HTML",
	}
	if replyMarkup != nil {
		payload["reply_markup"] = replyMarkup
	}

	_, err := c.postJSON(ctx, "/sendVideo", payload)
	return err
}

// SendPhoto sends a photo to the chat via direct URL.
func (c *Client) SendPhoto(ctx context.Context, chatID int64, photoURL string, caption string, replyMarkup *InlineKeyboardMarkup) error {
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"photo":      photoURL,
		"caption":    caption,
		"parse_mode": "HTML",
	}
	if replyMarkup != nil {
		payload["reply_markup"] = replyMarkup
	}

	_, err := c.postJSON(ctx, "/sendPhoto", payload)
	return err
}

// SendMediaGroup sends an album / carousel of media items.
func (c *Client) SendMediaGroup(ctx context.Context, chatID int64, media []InputMedia) error {
	payload := map[string]interface{}{
		"chat_id": chatID,
		"media":   media,
	}

	_, err := c.postJSON(ctx, "/sendMediaGroup", payload)
	return err
}

// SendLocalFile uploads a local media file as multipart/form-data.
func (c *Client) SendLocalFile(ctx context.Context, chatID int64, filePath, mediaType, caption string, replyMarkup *InlineKeyboardMarkup) error {
	endpoint := "/sendVideo"
	formFieldName := "video"
	if mediaType == "image" {
		endpoint = "/sendPhoto"
		formFieldName = "photo"
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open local file: %w", err)
	}
	defer file.Close()

	bodyBuf := &bytes.Buffer{}
	writer := multipart.NewWriter(bodyBuf)

	_ = writer.WriteField("chat_id", fmt.Sprintf("%d", chatID))
	_ = writer.WriteField("parse_mode", "HTML")
	if caption != "" {
		_ = writer.WriteField("caption", caption)
	}
	if replyMarkup != nil {
		markupBytes, _ := json.Marshal(replyMarkup)
		_ = writer.WriteField("reply_markup", string(markupBytes))
	}

	part, err := writer.CreateFormFile(formFieldName, filepath.Base(filePath))
	if err != nil {
		return fmt.Errorf("create form file: %w", err)
	}

	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("copy file bytes: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("close writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+endpoint, bodyBuf)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("upload local file: %w", err)
	}
	defer resp.Body.Close()

	var apiRes APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiRes); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if !apiRes.OK {
		return fmt.Errorf("telegram error %d: %s", apiRes.ErrorCode, apiRes.Description)
	}

	return nil
}

// SetWebhook configures the Telegram Bot API webhook.
func (c *Client) SetWebhook(ctx context.Context, webhookURL, secretToken string) (*APIResponse, error) {
	payload := map[string]interface{}{
		"url": webhookURL,
	}
	if secretToken != "" {
		payload["secret_token"] = secretToken
	}
	return c.postJSON(ctx, "/setWebhook", payload)
}

// GetWebhookInfo retrieves current webhook configuration.
func (c *Client) GetWebhookInfo(ctx context.Context) (*APIResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/getWebhookInfo", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apiRes APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiRes); err != nil {
		return nil, err
	}
	return &apiRes, nil
}

func (c *Client) postJSON(ctx context.Context, endpoint string, body interface{}) (*APIResponse, error) {
	jsonBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal json: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+endpoint, bytes.NewReader(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http post %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	var apiRes APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiRes); err != nil {
		return nil, fmt.Errorf("decode json from %s: %w", endpoint, err)
	}

	if !apiRes.OK {
		return &apiRes, fmt.Errorf("telegram API error %d: %s", apiRes.ErrorCode, apiRes.Description)
	}

	return &apiRes, nil
}
