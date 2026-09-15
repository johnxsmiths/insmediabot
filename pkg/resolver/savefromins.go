package resolver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type SaveFromInsProvider struct {
	apiURL string
	client *http.Client
}

func NewSaveFromInsProvider(apiURL string) *SaveFromInsProvider {
	if apiURL == "" {
		apiURL = "https://api.savefromins.com/api/contentsite_api/media/parse"
	}
	return &SaveFromInsProvider{
		apiURL: apiURL,
		client: &http.Client{Timeout: 25 * time.Second},
	}
}

func (p *SaveFromInsProvider) Name() string {
	return "savefromins"
}

func (p *SaveFromInsProvider) Resolve(ctx context.Context, igURL string) (*MediaResult, error) {
	payload := map[string]string{
		"url": igURL,
	}
	bodyData, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.apiURL, bytes.NewReader(bodyData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://savefromins.com/")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("savefromins status %d", resp.StatusCode)
	}

	var parsed struct {
		Code int `json:"code"`
		Data []struct {
			Media []struct {
				URL       string `json:"url"`
				Type      string `json:"type"`
				Thumbnail string `json:"thumbnail"`
			} `json:"media"`
			URL  string `json:"url"`
			Type string `json:"type"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode json: %w", err)
	}

	var items []MediaItem

	for _, d := range parsed.Data {
		if len(d.Media) > 0 {
			for _, m := range d.Media {
				if m.URL != "" {
					itemType := "video"
					if m.Type == "image" || strings.Contains(m.URL, ".jpg") {
						itemType = "image"
					}
					items = append(items, MediaItem{
						URL:      m.URL,
						Type:     itemType,
						MIMEType: "video/mp4",
					})
				}
			}
		} else if d.URL != "" {
			itemType := "video"
			if d.Type == "image" || strings.Contains(d.URL, ".jpg") {
				itemType = "image"
			}
			items = append(items, MediaItem{
				URL:      d.URL,
				Type:     itemType,
				MIMEType: "video/mp4",
			})
		}
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("no media items found in savefromins response")
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
