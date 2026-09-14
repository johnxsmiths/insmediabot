package resolver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// FastDLProvider handles fastdl.app / api-wh.fastdl.app / sssinstagram.com flow.
type FastDLProvider struct {
	name    string
	apiURL  string
	msecURL string
	client  *http.Client
}

// NewFastDLProvider creates a new FastDL or SSSInstagram resolver.
func NewFastDLProvider(name, apiURL, msecURL string) *FastDLProvider {
	if apiURL == "" {
		if name == "sssinstagram" {
			apiURL = "https://api-wh.sssinstagram.com/api/convert"
		} else {
			apiURL = "https://api-wh.fastdl.app/api/convert"
		}
	}
	if msecURL == "" {
		if name == "sssinstagram" {
			msecURL = "https://sssinstagram.com/msec"
		} else {
			msecURL = "https://fastdl.app/msec"
		}
	}

	return &FastDLProvider{
		name:    name,
		apiURL:  apiURL,
		msecURL: msecURL,
		client:  &http.Client{Timeout: 25 * time.Second},
	}
}

func (p *FastDLProvider) Name() string {
	return p.name
}

func (p *FastDLProvider) Resolve(ctx context.Context, igURL string) (*MediaResult, error) {
	ts := fmt.Sprintf("%d", time.Now().Unix())
	token := p.fetchToken(ctx)

	form := url.Values{}
	form.Set("url", igURL)
	form.Set("ts", ts)
	if token != "" {
		form.Set("token", token)
		form.Set("_token", token)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Origin", "https://fastdl.app")
	req.Header.Set("Referer", "https://fastdl.app/")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("post convert request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fastdl convert status %d", resp.StatusCode)
	}

	var parsed struct {
		URL   []interface{} `json:"url"`
		Meta  interface{}   `json:"meta"`
		HTML  string        `json:"html"`
		Data  interface{}   `json:"data"`
		Links []interface{} `json:"links"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode json response: %w", err)
	}

	var items []MediaItem

	for _, rawURL := range parsed.URL {
		if mapVal, ok := rawURL.(map[string]interface{}); ok {
			item := extractFastDLItem(mapVal)
			if item.URL != "" {
				items = append(items, item)
			}
		} else if strVal, ok := rawURL.(string); ok && strings.HasPrefix(strVal, "http") {
			items = append(items, MediaItem{
				URL:      strVal,
				Type:     "video",
				MIMEType: "video/mp4",
			})
		}
	}

	if len(items) == 0 {
		for _, rawLink := range parsed.Links {
			if mapVal, ok := rawLink.(map[string]interface{}); ok {
				item := extractFastDLItem(mapVal)
				if item.URL != "" {
					items = append(items, item)
				}
			}
		}
	}

	if len(items) == 0 && parsed.HTML != "" {
		items = extractFastDLFromHTML(parsed.HTML)
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("no media items extracted from %s", p.name)
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

func (p *FastDLProvider) fetchToken(ctx context.Context) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.msecURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	resp, err := p.client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var res struct {
		Token string `json:"token"`
		Key   string `json:"key"`
		Hash  string `json:"hash"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err == nil {
		if res.Token != "" {
			return res.Token
		}
		if res.Key != "" {
			return res.Key
		}
		if res.Hash != "" {
			return res.Hash
		}
	}
	return ""
}

func extractFastDLItem(itemMap map[string]interface{}) MediaItem {
	var mediaURL string
	var itemType string = "video"

	for _, k := range []string{"url", "download_url", "src", "link"} {
		if val, ok := itemMap[k].(string); ok && strings.HasPrefix(val, "http") {
			mediaURL = val
			break
		}
	}

	if typeVal, ok := itemMap["type"].(string); ok {
		if strings.Contains(strings.ToLower(typeVal), "photo") || strings.Contains(strings.ToLower(typeVal), "image") {
			itemType = "image"
		}
	}

	if mediaURL == "" {
		return MediaItem{}
	}

	mime := "video/mp4"
	if itemType == "image" {
		mime = "image/jpeg"
	}

	return MediaItem{
		URL:      mediaURL,
		Type:     itemType,
		MIMEType: mime,
	}
}

var hrefRegex = regexp.MustCompile(`href="(https?://[^"]+)"`)

func extractFastDLFromHTML(htmlContent string) []MediaItem {
	var items []MediaItem
	matches := hrefRegex.FindAllStringSubmatch(htmlContent, -1)
	for _, m := range matches {
		if len(m) > 1 {
			targetURL := m[1]
			if strings.Contains(targetURL, "cdninstagram.com") || strings.Contains(targetURL, "download") || strings.Contains(targetURL, "output") {
				itemType := "video"
				if strings.Contains(targetURL, ".jpg") || strings.Contains(targetURL, ".png") || strings.Contains(targetURL, ".webp") {
					itemType = "image"
				}
				items = append(items, MediaItem{
					URL:      targetURL,
					Type:     itemType,
					MIMEType: "video/mp4",
				})
			}
		}
	}
	return items
}
