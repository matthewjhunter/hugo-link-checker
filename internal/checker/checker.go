package checker

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/infodancer/hugo-link-checker/internal/scanner"
)

// CheckLinks validates all links in the provided files
func CheckLinks(files []*scanner.File, rootDir string, checkExternal bool, checkPublic bool, baseURL string, verbose bool) error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	for _, file := range files {
		for i := range file.Links {
			link := &file.Links[i]

			// Skip ignored links
			if link.Ignored {
				link.StatusCode = 200
				link.ErrorMessage = ""
				link.LastChecked = time.Now()
				continue
			}

			// Skip links with Hugo template syntax
			if strings.Contains(link.URL, "{{") || strings.Contains(link.URL, "}}") {
				link.StatusCode = 200
				link.ErrorMessage = ""
				link.LastChecked = time.Now()
				continue
			}

			if link.Type == scanner.LinkTypeExternal {
				if checkExternal {
					if strings.HasPrefix(link.URL, "mailto:") {
						err := checkMailtoLink(link)
						if err != nil {
							return fmt.Errorf("error checking mailto link %s: %v", link.URL, err)
						}
					} else {
						err := checkExternalLink(client, link)
						if err != nil {
							return fmt.Errorf("error checking external link %s: %v", link.URL, err)
						}
					}
				} else {
					// Skip external link checking, mark as OK
					link.StatusCode = 200
					link.ErrorMessage = ""
				}
			} else {
				err := checkInternalLink(link, rootDir, checkPublic, baseURL, client, verbose)
				if err != nil {
					return fmt.Errorf("error checking internal link %s: %v", link.URL, err)
				}
			}

			link.LastChecked = time.Now()
		}
	}

	return nil
}

func checkMailtoLink(link *scanner.Link) error {
	// Parse the mailto URL
	u, err := url.Parse(link.URL)
	if err != nil {
		link.StatusCode = 0
		link.ErrorMessage = "Invalid mailto URL"
		return nil
	}

	// Extract email address
	email := u.Opaque
	if email == "" {
		link.StatusCode = 0
		link.ErrorMessage = "No email address in mailto URL"
		return nil
	}

	// Extract domain from email
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		link.StatusCode = 0
		link.ErrorMessage = "Invalid email format"
		return nil
	}

	domain := parts[1]

	// Look up MX records for the domain
	_, err = net.LookupMX(domain)
	if err != nil {
		// If MX lookup fails, try A record lookup as fallback
		_, err = net.LookupHost(domain)
		if err != nil {
			link.StatusCode = 0
			link.ErrorMessage = fmt.Sprintf("Domain not found: %s", domain)
			return nil
		}
	}

	link.StatusCode = 200
	link.ErrorMessage = ""
	return nil
}

func checkExternalLink(client *http.Client, link *scanner.Link) error {
	resp, err := client.Head(link.URL)
	if err != nil {
		// Try GET if HEAD fails
		resp, err = client.Get(link.URL)
		if err != nil {
			link.StatusCode = 0
			link.ErrorMessage = err.Error()
			return nil
		}
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// Log the error but don't override the main function's return value
			fmt.Fprintf(os.Stderr, "Warning: failed to close response body: %v\n", closeErr)
		}
	}()

	link.StatusCode = resp.StatusCode
	if resp.StatusCode >= 400 {
		link.ErrorMessage = fmt.Sprintf("HTTP %d", resp.StatusCode)
	} else {
		link.ErrorMessage = ""
	}

	return nil
}

func checkInternalLink(link *scanner.Link, rootDir string, checkPublic bool, baseURL string, client *http.Client, verbose bool) error {
	// Clean and resolve the path
	linkPath := link.URL

	// Remove fragment identifier
	if idx := strings.Index(linkPath, "#"); idx != -1 {
		linkPath = linkPath[:idx]
	}

	// Remove query parameters
	if idx := strings.Index(linkPath, "?"); idx != -1 {
		linkPath = linkPath[:idx]
	}

	// Skip empty paths (fragment-only links)
	if linkPath == "" {
		link.StatusCode = 200
		link.ErrorMessage = ""
		return nil
	}

	// If base URL is provided, check the link online instead of locally
	if baseURL != "" {
		// Construct the full URL
		fullURL := strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(linkPath, "/")

		// Create a temporary link to check online
		tempLink := &scanner.Link{URL: fullURL}
		err := checkExternalLink(client, tempLink)
		if err != nil {
			return err
		}

		// Copy the results back to the original link
		link.StatusCode = tempLink.StatusCode
		link.ErrorMessage = tempLink.ErrorMessage
	} else {
		// Check if file exists locally using Hugo conventions
		var found bool
		var checkedPaths []string

		if checkPublic {
			// Check in Hugo's public directory for built site files
			found, checkedPaths = checkPublicFileVerbose(linkPath, rootDir, verbose)
		} else {
			// Check using standard Hugo source conventions
			found, checkedPaths = checkHugoFile(linkPath, rootDir, verbose)
		}

		if found {
			link.StatusCode = 200
			link.ErrorMessage = ""
		} else {
			link.StatusCode = 404
			if verbose && len(checkedPaths) > 0 {
				link.ErrorMessage = fmt.Sprintf("File not found. Checked paths: %s", strings.Join(checkedPaths, ", "))
			} else {
				link.ErrorMessage = "File not found"
			}
		}
	}

	return nil
}

// checkHugoFile checks if a file exists using Hugo's conventions and optionally returns checked paths
func checkHugoFile(linkPath string, rootDir string, verbose bool) (bool, []string) {
	// Clean the path
	linkPath = strings.TrimPrefix(linkPath, "/")

	// Detect if we're scanning from within a Hugo content directory
	// and adjust the root to be the Hugo site root
	hugoSiteRoot := rootDir
	if strings.Contains(rootDir, "/content/") {
		// Find the Hugo site root by going up to the directory containing "content"
		parts := strings.Split(rootDir, "/content/")
		if len(parts) > 0 {
			hugoSiteRoot = parts[0]
		}
	}

	// List of possible file locations to check
	var candidatePaths []string

	// 1. Direct path in root directory
	candidatePaths = append(candidatePaths, filepath.Join(rootDir, linkPath))

	// 2. If we detected a Hugo site root different from rootDir, check there too
	if hugoSiteRoot != rootDir {
		candidatePaths = append(candidatePaths, filepath.Join(hugoSiteRoot, linkPath))
	}

	// 3. Static directory (for images and other assets)
	candidatePaths = append(candidatePaths, filepath.Join(rootDir, "static", linkPath))
	candidatePaths = append(candidatePaths, filepath.Join(hugoSiteRoot, "static", linkPath))

	// 4. Content directory (for markdown files)
	candidatePaths = append(candidatePaths, filepath.Join(rootDir, "content", linkPath))
	candidatePaths = append(candidatePaths, filepath.Join(hugoSiteRoot, "content", linkPath))

	// 5. Hugo URL transformation: /example/ -> content/example.md or content/example/index.md
	if strings.HasSuffix(linkPath, "/") {
		basePath := strings.TrimSuffix(linkPath, "/")

		// Try content/example.md
		candidatePaths = append(candidatePaths, filepath.Join(rootDir, "content", basePath+".md"))
		candidatePaths = append(candidatePaths, filepath.Join(hugoSiteRoot, "content", basePath+".md"))

		// Try content/example/index.md
		candidatePaths = append(candidatePaths, filepath.Join(rootDir, "content", basePath, "index.md"))
		candidatePaths = append(candidatePaths, filepath.Join(hugoSiteRoot, "content", basePath, "index.md"))

		// Try content/example/_index.md (for list pages)
		candidatePaths = append(candidatePaths, filepath.Join(rootDir, "content", basePath, "_index.md"))
		candidatePaths = append(candidatePaths, filepath.Join(hugoSiteRoot, "content", basePath, "_index.md"))

		// Try direct path as .md file (for when root is already in content)
		candidatePaths = append(candidatePaths, filepath.Join(rootDir, basePath+".md"))
		candidatePaths = append(candidatePaths, filepath.Join(hugoSiteRoot, basePath+".md"))

		// Try direct path with index.md (for when root is already in content)
		candidatePaths = append(candidatePaths, filepath.Join(rootDir, basePath, "index.md"))
		candidatePaths = append(candidatePaths, filepath.Join(hugoSiteRoot, basePath, "index.md"))

		// Try direct path with _index.md (for when root is already in content)
		candidatePaths = append(candidatePaths, filepath.Join(rootDir, basePath, "_index.md"))
		candidatePaths = append(candidatePaths, filepath.Join(hugoSiteRoot, basePath, "_index.md"))
	}

	// 6. If no trailing slash, also try the Hugo transformations
	if !strings.HasSuffix(linkPath, "/") && !strings.Contains(filepath.Base(linkPath), ".") {
		// Try content/example.md
		candidatePaths = append(candidatePaths, filepath.Join(rootDir, "content", linkPath+".md"))
		candidatePaths = append(candidatePaths, filepath.Join(hugoSiteRoot, "content", linkPath+".md"))

		// Try content/example/index.md
		candidatePaths = append(candidatePaths, filepath.Join(rootDir, "content", linkPath, "index.md"))
		candidatePaths = append(candidatePaths, filepath.Join(hugoSiteRoot, "content", linkPath, "index.md"))

		// Try content/example/_index.md
		candidatePaths = append(candidatePaths, filepath.Join(rootDir, "content", linkPath, "_index.md"))
		candidatePaths = append(candidatePaths, filepath.Join(hugoSiteRoot, "content", linkPath, "_index.md"))

		// Try direct path as .md file (for when root is already in content)
		candidatePaths = append(candidatePaths, filepath.Join(rootDir, linkPath+".md"))
		candidatePaths = append(candidatePaths, filepath.Join(hugoSiteRoot, linkPath+".md"))

		// Try direct path with index.md (for when root is already in content)
		candidatePaths = append(candidatePaths, filepath.Join(rootDir, linkPath, "index.md"))
		candidatePaths = append(candidatePaths, filepath.Join(hugoSiteRoot, linkPath, "index.md"))

		// Try direct path with _index.md (for when root is already in content)
		candidatePaths = append(candidatePaths, filepath.Join(rootDir, linkPath, "_index.md"))
		candidatePaths = append(candidatePaths, filepath.Join(hugoSiteRoot, linkPath, "_index.md"))
	}

	// Deduplicate candidate paths
	seen := make(map[string]bool)
	var uniquePaths []string
	for _, path := range candidatePaths {
		if !seen[path] {
			seen[path] = true
			uniquePaths = append(uniquePaths, path)
		}
	}

	// Check each candidate path (case-insensitive for source files)
	var checkedPaths []string
	for _, path := range uniquePaths {
		if verbose {
			checkedPaths = append(checkedPaths, path)
		}

		// First try exact match
		if _, err := os.Stat(path); err == nil {
			return true, checkedPaths
		}

		// If exact match fails and this looks like a source file path, try case-insensitive matching
		if isSourceFilePath(path) {
			if found := findCaseInsensitiveFile(path); found != "" {
				if verbose {
					checkedPaths = append(checkedPaths, found)
				}
				return true, checkedPaths
			}
		}
	}

	return false, checkedPaths
}

// checkPublicFileVerbose checks if a file exists in Hugo's public directory and optionally returns checked paths
func checkPublicFileVerbose(linkPath string, rootDir string, verbose bool) (bool, []string) {
	// Clean the path
	linkPath = strings.TrimPrefix(linkPath, "/")

	// Detect Hugo site root
	hugoSiteRoot := rootDir
	if strings.Contains(rootDir, "/content/") {
		// Find the Hugo site root by going up to the directory containing "content"
		parts := strings.Split(rootDir, "/content/")
		if len(parts) > 0 {
			hugoSiteRoot = parts[0]
		}
	}

	// List of possible file locations to check in public directory
	var candidatePaths []string

	// 1. Direct path in public directory
	candidatePaths = append(candidatePaths, filepath.Join(hugoSiteRoot, "public", linkPath))

	// 2. If linkPath ends with /, try index.html
	if strings.HasSuffix(linkPath, "/") {
		candidatePaths = append(candidatePaths, filepath.Join(hugoSiteRoot, "public", linkPath, "index.html"))
	}

	// 3. If linkPath doesn't have extension, try adding index.html
	if !strings.Contains(filepath.Base(linkPath), ".") {
		candidatePaths = append(candidatePaths, filepath.Join(hugoSiteRoot, "public", linkPath, "index.html"))
		// Also try with trailing slash and index.html
		candidatePaths = append(candidatePaths, filepath.Join(hugoSiteRoot, "public", linkPath+"/index.html"))
	}

	// 4. Try common Hugo output formats
	if !strings.Contains(filepath.Base(linkPath), ".") {
		candidatePaths = append(candidatePaths, filepath.Join(hugoSiteRoot, "public", linkPath+".html"))
	}

	// Deduplicate candidate paths
	seen := make(map[string]bool)
	var uniquePaths []string
	for _, path := range candidatePaths {
		if !seen[path] {
			seen[path] = true
			uniquePaths = append(uniquePaths, path)
		}
	}

	// Check each candidate path
	var checkedPaths []string
	for _, path := range uniquePaths {
		if verbose {
			checkedPaths = append(checkedPaths, path)
		}
		if _, err := os.Stat(path); err == nil {
			return true, checkedPaths
		}
	}

	return false, checkedPaths
}

// isSourceFilePath checks if a path looks like it's for a Hugo source file
func isSourceFilePath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".md" || ext == ".markdown" || strings.Contains(path, "/content/")
}

// findCaseInsensitiveFile attempts to find a file with case-insensitive matching
func findCaseInsensitiveFile(targetPath string) string {
	dir := filepath.Dir(targetPath)
	targetBase := filepath.Base(targetPath)

	// Read the directory
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	// Look for case-insensitive match
	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), targetBase) {
			return filepath.Join(dir, entry.Name())
		}
	}

	return ""
}

// CountBrokenLinks returns the number of broken links across all files
func CountBrokenLinks(files []*scanner.File) int {
	count := 0
	for _, file := range files {
		for _, link := range file.Links {
			// Skip ignored links when counting broken links
			if link.Ignored {
				continue
			}
			if link.StatusCode >= 400 || (link.StatusCode == 0 && link.ErrorMessage != "") {
				count++
			}
		}
	}
	return count
}
