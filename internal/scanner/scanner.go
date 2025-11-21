package scanner

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

// LinkType represents whether a link is internal or external
type LinkType int

const (
	LinkTypeInternal LinkType = iota
	LinkTypeExternal
)

// Link represents a link found in a file
type Link struct {
	URL          string    `json:"url"`
	Type         LinkType  `json:"type"`
	LastChecked  time.Time `json:"last_checked"`
	StatusCode   int       `json:"status_code"`
	ErrorMessage string    `json:"error_message,omitempty"`
}

// File represents a file and its links
type File struct {
	Path          string `json:"path"`
	CanonicalPath string `json:"canonical_path"`
	Links         []Link `json:"links"`
}

// isInternalLink determines if a link is internal (relative) or external
func isInternalLink(linkURL string) bool {
	// Parse the URL
	u, err := url.Parse(linkURL)
	if err != nil {
		// If we can't parse it, treat as internal for safety
		return true
	}
	
	// If it has a scheme (http, https, etc.) or host, it's external
	if u.Scheme != "" || u.Host != "" {
		return false
	}
	
	// Otherwise it's a relative/internal link
	return true
}

// NewLink creates a new Link with the appropriate type
func NewLink(linkURL string) Link {
	linkType := LinkTypeInternal
	if !isInternalLink(linkURL) {
		linkType = LinkTypeExternal
	}
	
	return Link{
		URL:  linkURL,
		Type: linkType,
	}
}

// ParseLinksFromFile reads a file and extracts all links using regex
func ParseLinksFromFile(file *File) error {
	// Regular expressions for different link formats
	// Markdown: [text](url), <url>, [ref]: url
	// HTML: <a href="url">, <link href="url">
	linkRegexes := []*regexp.Regexp{
		regexp.MustCompile(`\[([^\]]*)\]\(([^)]+)\)`),           // [text](url) - markdown
		regexp.MustCompile(`<(https?://[^>]+)>`),                // <http://example.com> - markdown autolinks
		regexp.MustCompile(`^\s*\[([^\]]+)\]:\s*(.+)$`),         // [ref]: url - markdown reference definitions
		regexp.MustCompile(`<a\s+[^>]*href\s*=\s*["']([^"']+)["'][^>]*>`), // <a href="url"> - HTML
		regexp.MustCompile(`<link\s+[^>]*href\s*=\s*["']([^"']+)["'][^>]*>`), // <link href="url"> - HTML
	}
	
	// Open the file
	f, err := os.Open(file.Path)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", file.Path, err)
	}
	defer f.Close()
	
	// Track unique links to avoid duplicates
	linkMap := make(map[string]bool)
	
	// Read file line by line
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		
		// Apply each regex to find links
		for _, regex := range linkRegexes {
			matches := regex.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				var linkURL string
				if len(match) >= 3 {
					// For [text](url) format, URL is in match[2]
					linkURL = strings.TrimSpace(match[2])
				} else if len(match) >= 2 {
					// For <url> format, URL is in match[1]
					linkURL = strings.TrimSpace(match[1])
				}
				
				if linkURL == "" {
					continue
				}
				
				// Remove any title part from the URL (everything after first space or quote)
				if spaceIdx := strings.Index(linkURL, " "); spaceIdx != -1 {
					linkURL = linkURL[:spaceIdx]
				}
				if quoteIdx := strings.Index(linkURL, `"`); quoteIdx != -1 {
					linkURL = linkURL[:quoteIdx]
				}
				
				linkURL = strings.TrimSpace(linkURL)
				
				// Skip empty URLs or fragment-only links
				if linkURL == "" || linkURL == "#" {
					continue
				}
				
				// Check if we've already seen this link
				if linkMap[linkURL] {
					continue
				}
				linkMap[linkURL] = true
				
				// Create and add the link
				link := NewLink(linkURL)
				file.Links = append(file.Links, link)
			}
		}
	}
	
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading file %s: %w", file.Path, err)
	}
	
	return nil
}
