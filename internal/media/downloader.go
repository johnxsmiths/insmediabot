package media

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Downloader manages safe downloading of media items to ephemeral serverless storage.
type Downloader struct {
	client   *http.Client
	maxBytes int64
}

// DownloadedFile wraps an ephemeral temporary file with its metadata and safe cleanup.
type DownloadedFile struct {
	Path        string
	Size        int64
	ContentType string
}

// Close removes the temporary local file from the disk.
func (f *DownloadedFile) Close() {
	if f != nil && f.Path != "" {
		_ = os.Remove(f.Path)
	}
}

// NewDownloader creates a new media downloader with safety bounds.
func NewDownloader(maxBytes int64) *Downloader {
	if maxBytes <= 0 {
		maxBytes = 50 * 1024 * 1024 // 50 MB max limit for Telegram Bot API uploads
	}
	return &Downloader{
		client: &http.Client{
			Timeout: 40 * time.Second,
		},
		maxBytes: maxBytes,
	}
}

// Download saves a media URL to an ephemeral temporary file on the local OS temporary filesystem.
func (d *Downloader) Download(ctx context.Context, mediaURL, filename string) (*DownloadedFile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mediaURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create download request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://www.instagram.com/")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute download request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("download failed with HTTP %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")

	// Create temp file in system temp dir
	tmpFile, err := os.CreateTemp("", "slmedia-*"+filepath.Ext(filename))
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	defer tmpFile.Close()

	// Guard against excessive file size
	limitedReader := io.LimitReader(resp.Body, d.maxBytes+1)
	written, err := io.Copy(tmpFile, limitedReader)
	if err != nil {
		_ = os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("write media: %w", err)
	}

	if written > d.maxBytes {
		_ = os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("media size exceeds allowed limit of %d bytes", d.maxBytes)
	}

	return &DownloadedFile{
		Path:        tmpFile.Name(),
		Size:        written,
		ContentType: contentType,
	}, nil
}
