package resolver

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// SnapSaveProvider handles snapsave.app / v3.saveclip.app requests.
type SnapSaveProvider struct {
	name   string
	apiURL string
	client *http.Client
}

// NewSnapSaveProvider creates a new SnapSave or SaveClip resolver.
func NewSnapSaveProvider(name, apiURL string) *SnapSaveProvider {
	if apiURL == "" {
		if name == "saveclip" {
			apiURL = "https://v3.saveclip.app/api/ajaxSearch"
		} else {
			apiURL = "https://snapsave.app/action.php?lang=en"
		}
	}
	return &SnapSaveProvider{
		name:   name,
		apiURL: apiURL,
		client: &http.Client{Timeout: 25 * time.Second},
	}
}

func (p *SnapSaveProvider) Name() string {
	return p.name
}

func (p *SnapSaveProvider) Resolve(ctx context.Context, igURL string) (*MediaResult, error) {
	form := url.Values{}
	form.Set("url", igURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://snapsave.app/")
	req.Header.Set("Origin", "https://snapsave.app")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do post: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	bodyStr := string(bodyBytes)
	items := extractLinksFromSnapSave(bodyStr)
	if len(items) == 0 {
		return nil, fmt.Errorf("no media items found in %s response", p.name)
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

var snapDownloadRegex = regexp.MustCompile(`(?:href|src)=["'](https?://[^"']+)["']`)

func extractLinksFromSnapSave(text string) []MediaItem {
	var items []MediaItem
	seen := make(map[string]bool)

	matches := snapDownloadRegex.FindAllStringSubmatch(text, -1)
	for _, m := range matches {
		if len(m) > 1 {
			u := m[1]
			if (strings.Contains(u, "cdninstagram.com") || strings.Contains(u, "download.php") || strings.Contains(u, "download")) && !seen[u] {
				seen[u] = true
				itemType := "video"
				if strings.Contains(u, ".jpg") || strings.Contains(u, ".jpeg") || strings.Contains(u, ".png") || strings.Contains(u, ".webp") {
					itemType = "image"
				}
				items = append(items, MediaItem{
					URL:      u,
					Type:     itemType,
					MIMEType: "video/mp4",
				})
			}
		}
	}
	return items
}
