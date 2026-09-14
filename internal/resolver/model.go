package resolver

import "context"

// MediaResult represents the normalized response from any Instagram provider.
type MediaResult struct {
	SourceURL string      `json:"source_url"`
	Title     string      `json:"title"`
	MediaType string      `json:"media_type"` // video, image, carousel
	Items     []MediaItem `json:"items"`
}

// MediaItem represents an individual video or image file extracted from the post.
type MediaItem struct {
	URL      string `json:"url"`
	Type     string `json:"type"` // "video" or "image"
	MIMEType string `json:"mime_type,omitempty"`
	Filename string `json:"filename,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Quality  string `json:"quality,omitempty"`
}

// Provider defines the interface for resolving Instagram media URLs.
type Provider interface {
	Name() string
	Resolve(ctx context.Context, igURL string) (*MediaResult, error)
}
