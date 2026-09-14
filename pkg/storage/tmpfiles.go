package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// TmpFilesStorage implements free temporary file uploading via tmpfiles.org.
// tmpfiles.org is 100% free, requires no API key, and auto-expires files.
type TmpFilesStorage struct {
	apiURL string
	client *http.Client
}

// NewTmpFilesStorage returns a new TmpFilesStorage instance.
func NewTmpFilesStorage(apiURL string) *TmpFilesStorage {
	if apiURL == "" {
		apiURL = "https://tmpfiles.org/api/v1/upload"
	}
	return &TmpFilesStorage{
		apiURL: apiURL,
		client: &http.Client{Timeout: 45 * time.Second},
	}
}

func (s *TmpFilesStorage) Name() string {
	return "tmpfiles"
}

// Upload uploads a temporary file and returns a direct downloadable URL.
func (s *TmpFilesStorage) Upload(ctx context.Context, reader io.Reader, filename, contentType string) (string, error) {
	if filename == "" {
		filename = "instagram_media.mp4"
	}

	bodyBuf := &bytes.Buffer{}
	writer := multipart.NewWriter(bodyBuf)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}

	if _, err := io.Copy(part, reader); err != nil {
		return "", fmt.Errorf("copy media to form: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.apiURL, bodyBuf)
	if err != nil {
		return "", fmt.Errorf("create upload request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("do upload request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("upload failed with status: %d", resp.StatusCode)
	}

	var res struct {
		Status string `json:"status"`
		Data   struct {
			URL string `json:"url"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", fmt.Errorf("decode upload response: %w", err)
	}

	if res.Data.URL == "" {
		return "", fmt.Errorf("no direct url returned from tmpfiles")
	}

	// tmpfiles.org URLs are returned as: https://tmpfiles.org/123456/sample.mp4
	// To convert to direct streamable URL for Telegram: https://tmpfiles.org/dl/123456/sample.mp4
	directURL := res.Data.URL
	if strings.Contains(directURL, "tmpfiles.org/") && !strings.Contains(directURL, "tmpfiles.org/dl/") {
		directURL = strings.Replace(directURL, "tmpfiles.org/", "tmpfiles.org/dl/", 1)
	}

	return directURL, nil
}

// Delete is handled by TTL on tmpfiles.org; best effort call.
func (s *TmpFilesStorage) Delete(ctx context.Context, fileIDOrURL string) error {
	// tmpfiles.org auto-deletes files automatically based on expiration policy
	return nil
}
