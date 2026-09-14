package resolver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// IGExportProvider resolves Instagram posts/reels using the igexport API.
// Live Endpoint: https://igexport.com/api/ig-photo/?url=...
type IGExportProvider struct {
	apiURL string
	client *http.Client
}

// NewIGExportProvider initializes an IGExportProvider.
func NewIGExportProvider(apiURL string) *IGExportProvider {
	if apiURL == "" {
		apiURL = "https://igexport.com/api/ig-photo/"
	}
	return &IGExportProvider{
		apiURL: apiURL,
		client: &http.Client{Timeout: 20 * time.Second},
	}
}

func (p *IGExportProvider) Name() string {
	return "igexport"
}

func (p *IGExportProvider) Resolve(ctx context.Context, igURL string) (*MediaResult, error) {
	endpoint := fmt.Sprintf("%s?url=%s", p.apiURL, url.QueryEscape(igURL))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Referer", "https://igexport.com/")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status code %d from igexport", resp.StatusCode)
	}

	var root struct {
		OK    bool `json:"ok"`
		Media struct {
			Shortcode string `json:"shortcode"`
			Items     []struct {
				Type         string `json:"type"`
				URL          string `json:"url"`
				ThumbnailURL string `json:"thumbnailUrl"`
				Filename     string `json:"filename"`
			} `json:"items"`
		} `json:"media"`
		Data interface{} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&root); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}

	var items []MediaItem

	// 1. Primary path: root.media.items
	if len(root.Media.Items) > 0 {
		for _, raw := range root.Media.Items {
			if raw.URL != "" {
				itemType := "video"
				mimeType := "video/mp4"
				if raw.Type == "photo" || raw.Type == "image" || strings.Contains(raw.URL, ".jpg") {
					itemType = "image"
					mimeType = "image/jpeg"
				}
				items = append(items, MediaItem{
					URL:          raw.URL,
					Type:         itemType,
					ThumbnailURL: raw.ThumbnailURL,
					MIMEType:     mimeType,
					Filename:     raw.Filename,
				})
			}
		}
	}

	// 2. Fallback path if returned in root.data
	if len(items) == 0 && root.Data != nil {
		if list, ok := root.Data.([]interface{}); ok {
			for _, elem := range list {
				if m, ok := elem.(map[string]interface{}); ok {
					item := extractIGExportItem(m)
					if item.URL != "" {
						items = append(items, item)
					}
				}
			}
		} else if m, ok := root.Data.(map[string]interface{}); ok {
			item := extractIGExportItem(m)
			if item.URL != "" {
				items = append(items, item)
			}
		}
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("no downloadable media found in igexport response")
	}

	mediaType := "video"
	if len(items) > 1 {
		mediaType = "carousel"
	} else if items[0].Type == "image" {
		mediaType = "image"
	}

	return &MediaResult{
		SourceURL: igURL,
		MediaType: mediaType,
		Items:     items,
	}, nil
}

func extractIGExportItem(data map[string]interface{}) MediaItem {
	var mediaURL string
	var itemType string = "video"

	for _, key := range []string{"url", "video_url", "videoUrl", "download_url", "media_url", "image_url"} {
		if val, ok := data[key].(string); ok && strings.HasPrefix(val, "http") {
			mediaURL = val
			if strings.Contains(key, "image") || strings.Contains(val, ".jpg") || strings.Contains(val, ".jpeg") {
				itemType = "image"
			}
			break
		}
	}

	if mediaURL == "" {
		return MediaItem{}
	}

	mimeType := "video/mp4"
	if itemType == "image" {
		mimeType = "image/jpeg"
	}

	return MediaItem{
		URL:      mediaURL,
		Type:     itemType,
		MIMEType: mimeType,
	}
}
