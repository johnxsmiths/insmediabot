package resolver

import (
	"net/url"
	"regexp"
	"strings"
)

var igHostRegex = regexp.MustCompile(`^(?:www\.)?(?:instagram\.com|instagr\.am)$`)
var igPathRegex = regexp.MustCompile(`^/(?:p|reel|reels|tv|stories)/([A-Za-z0-9_-]+)`)

// IsInstagramURL checks if the provided string is a valid Instagram link.
func IsInstagramURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if !igHostRegex.MatchString(host) {
		return false
	}
	return igPathRegex.MatchString(parsed.Path)
}

// NormalizeInstagramURL strips tracking parameters (igsh, utm_*, etc.) and returns canonical URL.
func NormalizeInstagramURL(rawURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", err
	}

	// Canonicalize host to https://www.instagram.com
	parsed.Scheme = "https"
	parsed.Host = "www.instagram.com"

	// Preserve only the content path
	matches := igPathRegex.FindStringSubmatch(parsed.Path)
	if len(matches) >= 2 {
		shortcode := matches[1]
		if strings.HasPrefix(parsed.Path, "/reel") || strings.HasPrefix(parsed.Path, "/reels") {
			parsed.Path = "/reel/" + shortcode + "/"
		} else if strings.HasPrefix(parsed.Path, "/tv") {
			parsed.Path = "/tv/" + shortcode + "/"
		} else {
			parsed.Path = "/p/" + shortcode + "/"
		}
	}

	// Remove all query parameters and fragments for privacy and clean fetching
	parsed.RawQuery = ""
	parsed.Fragment = ""

	return parsed.String(), nil
}

// ExtractShortcode gets the identifier of the post/reel.
func ExtractShortcode(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	matches := igPathRegex.FindStringSubmatch(parsed.Path)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}
