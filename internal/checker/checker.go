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
func CheckLinks(files []*scanner.File, rootDir string, checkExternal bool, baseURL string, verbose bool) error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	for _, file := range files {
		for i := range file.Links {
			link := &file.Links[i]
			
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
					// Skip external link checking, mark as unchecked
					link.StatusCode = 0
					link.ErrorMessage = "External link checking disabled"
				}
			} else {
				err := checkInternalLink(link, rootDir, baseURL, client, verbose)
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
	defer resp.Body.Close()
	
	link.StatusCode = resp.StatusCode
	if resp.StatusCode >= 400 {
		link.ErrorMessage = fmt.Sprintf("HTTP %d", resp.StatusCode)
	} else {
		link.ErrorMessage = ""
	}
	
	return nil
}

func checkInternalLink(link *scanner.Link, rootDir string, baseURL string, client *http.Client, verbose bool) error {
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
		found, checkedPaths := checkHugoFileVerbose(linkPath, rootDir, verbose)
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

// checkHugoFile checks if a file exists using Hugo's conventions
func checkHugoFile(linkPath string, rootDir string) bool {
	found, _ := checkHugoFileVerbose(linkPath, rootDir, false)
	return found
}

// checkHugoFileVerbose checks if a file exists using Hugo's conventions and optionally returns checked paths
func checkHugoFileVerbose(linkPath string, rootDir string, verbose bool) (bool, []string) {
	// Clean the path
	linkPath = strings.TrimPrefix(linkPath, "/")
	
	// List of possible file locations to check
	var candidatePaths []string
	
	// 1. Direct path in root directory
	candidatePaths = append(candidatePaths, filepath.Join(rootDir, linkPath))
	
	// 2. Static directory (for images and other assets)
	candidatePaths = append(candidatePaths, filepath.Join(rootDir, "static", linkPath))
	
	// 3. Content directory (for markdown files)
	candidatePaths = append(candidatePaths, filepath.Join(rootDir, "content", linkPath))
	
	// 4. Hugo URL transformation: /example/ -> content/example.md or content/example/index.md
	if strings.HasSuffix(linkPath, "/") {
		basePath := strings.TrimSuffix(linkPath, "/")
		
		// Try content/example.md
		candidatePaths = append(candidatePaths, filepath.Join(rootDir, "content", basePath+".md"))
		
		// Try content/example/index.md
		candidatePaths = append(candidatePaths, filepath.Join(rootDir, "content", basePath, "index.md"))
		
		// Try content/example/_index.md (for list pages)
		candidatePaths = append(candidatePaths, filepath.Join(rootDir, "content", basePath, "_index.md"))
		
		// Try direct path as .md file (for when root is already in content)
		candidatePaths = append(candidatePaths, filepath.Join(rootDir, basePath+".md"))
		
		// Try direct path with index.md (for when root is already in content)
		candidatePaths = append(candidatePaths, filepath.Join(rootDir, basePath, "index.md"))
		
		// Try direct path with _index.md (for when root is already in content)
		candidatePaths = append(candidatePaths, filepath.Join(rootDir, basePath, "_index.md"))
	}
	
	// 5. If no trailing slash, also try the Hugo transformations
	if !strings.HasSuffix(linkPath, "/") && !strings.Contains(filepath.Base(linkPath), ".") {
		// Try content/example.md
		candidatePaths = append(candidatePaths, filepath.Join(rootDir, "content", linkPath+".md"))
		
		// Try content/example/index.md
		candidatePaths = append(candidatePaths, filepath.Join(rootDir, "content", linkPath, "index.md"))
		
		// Try content/example/_index.md
		candidatePaths = append(candidatePaths, filepath.Join(rootDir, "content", linkPath, "_index.md"))
		
		// Try direct path as .md file (for when root is already in content)
		candidatePaths = append(candidatePaths, filepath.Join(rootDir, linkPath+".md"))
		
		// Try direct path with index.md (for when root is already in content)
		candidatePaths = append(candidatePaths, filepath.Join(rootDir, linkPath, "index.md"))
		
		// Try direct path with _index.md (for when root is already in content)
		candidatePaths = append(candidatePaths, filepath.Join(rootDir, linkPath, "_index.md"))
	}
	
	// Check each candidate path
	var checkedPaths []string
	for _, path := range candidatePaths {
		if verbose {
			checkedPaths = append(checkedPaths, path)
		}
		if _, err := os.Stat(path); err == nil {
			return true, checkedPaths
		}
	}
	
	return false, checkedPaths
}

// CountBrokenLinks returns the number of broken links across all files
func CountBrokenLinks(files []*scanner.File) int {
	count := 0
	for _, file := range files {
		for _, link := range file.Links {
			if link.StatusCode >= 400 || (link.StatusCode == 0 && link.ErrorMessage != "") {
				count++
			}
		}
	}
	return count
}
