package reporter

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/infodancer/hugo-link-checker/internal/scanner"
)

type ReportFormat string

const (
	FormatText ReportFormat = "text"
	FormatJSON ReportFormat = "json"
	FormatHTML ReportFormat = "html"
)

type ReportOptions struct {
	Format     ReportFormat
	OutputFile string
}

type JSONReport struct {
	GeneratedAt time.Time         `json:"generated_at"`
	Summary     ReportSummary     `json:"summary"`
	Links       []UniqueLink      `json:"links"`
}

type ReportSummary struct {
	TotalFiles       int `json:"total_files"`
	TotalLinks       int `json:"total_links"`
	UniqueLinks      int `json:"unique_links"`
	BrokenLinks      int `json:"broken_links"`
	InternalLinks    int `json:"internal_links"`
	ExternalLinks    int `json:"external_links"`
}

type UniqueLink struct {
	URL          string    `json:"url"`
	Type         string    `json:"type"`
	StatusCode   int       `json:"status_code"`
	ErrorMessage string    `json:"error_message,omitempty"`
	LastChecked  time.Time `json:"last_checked"`
	FoundInFiles []string  `json:"found_in_files"`
}

// GenerateReport creates a report in the specified format
func GenerateReport(files []*scanner.File, options ReportOptions) error {
	var writer io.Writer = os.Stdout
	
	if options.OutputFile != "" {
		file, err := os.Create(options.OutputFile)
		if err != nil {
			return fmt.Errorf("failed to create output file: %v", err)
		}
		defer file.Close()
		writer = file
	}
	
	switch options.Format {
	case FormatJSON:
		return generateJSONReport(files, writer)
	case FormatHTML:
		return generateHTMLReport(files, writer)
	default:
		return generateTextReport(files, writer)
	}
}

func generateTextReport(files []*scanner.File, writer io.Writer) error {
	summary := calculateSummary(files)
	
	fmt.Fprintf(writer, "Hugo Link Checker Report\n")
	fmt.Fprintf(writer, "========================\n")
	fmt.Fprintf(writer, "Generated: %s\n\n", time.Now().Format(time.RFC3339))
	
	fmt.Fprintf(writer, "Summary:\n")
	fmt.Fprintf(writer, "  Files scanned: %d\n", summary.TotalFiles)
	fmt.Fprintf(writer, "  Total links: %d\n", summary.TotalLinks)
	fmt.Fprintf(writer, "  Unique links: %d\n", summary.UniqueLinks)
	fmt.Fprintf(writer, "  Broken links: %d\n", summary.BrokenLinks)
	fmt.Fprintf(writer, "  Internal links: %d\n", summary.InternalLinks)
	fmt.Fprintf(writer, "  External links: %d\n\n", summary.ExternalLinks)
	
	// Filter files to only show markdown/HTML files with broken links
	for _, file := range files {
		if !isMarkdownOrHTML(file.Path) {
			continue
		}
		
		// Check if this file has any broken links
		var brokenLinks []scanner.Link
		for _, link := range file.Links {
			if link.StatusCode >= 400 || (link.StatusCode == 0 && link.ErrorMessage != "") {
				brokenLinks = append(brokenLinks, link)
			}
		}
		
		// Only show files that have broken links
		if len(brokenLinks) == 0 {
			continue
		}
		
		fmt.Fprintf(writer, "File: %s\n", file.Path)
		fmt.Fprintf(writer, "  Canonical: %s\n", file.CanonicalPath)
		fmt.Fprintf(writer, "  Links (broken/total): %d/%d\n", len(brokenLinks), len(file.Links))
		
		// Only show broken links
		for _, link := range brokenLinks {
			status := "BROKEN"
			if link.ErrorMessage != "" {
				status = fmt.Sprintf("BROKEN (%s)", link.ErrorMessage)
			}
			
			linkType := "internal"
			if link.Type == scanner.LinkTypeExternal {
				linkType = "external"
			}
			
			fmt.Fprintf(writer, "    %s [%s] - %s\n", link.URL, linkType, status)
		}
		fmt.Fprintf(writer, "\n")
	}
	
	return nil
}

func generateJSONReport(files []*scanner.File, writer io.Writer) error {
	summary := calculateSummary(files)
	uniqueLinks := getUniqueLinks(files)
	
	report := JSONReport{
		GeneratedAt: time.Now(),
		Summary:     summary,
		Links:       uniqueLinks,
	}
	
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func generateHTMLReport(files []*scanner.File, writer io.Writer) error {
	summary := calculateSummary(files)
	
	fmt.Fprintf(writer, `<!DOCTYPE html>
<html>
<head>
    <title>Hugo Link Checker Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .summary { background: #f5f5f5; padding: 15px; border-radius: 5px; margin-bottom: 20px; }
        .file { margin-bottom: 20px; border: 1px solid #ddd; padding: 15px; border-radius: 5px; }
        .file h3 { margin-top: 0; color: #333; }
        .link { margin: 5px 0; padding: 5px; }
        .link.broken { background: #ffe6e6; color: #d00; }
        .link.ok { background: #e6ffe6; color: #060; }
        .internal { font-style: italic; }
        .external { font-weight: bold; }
    </style>
</head>
<body>
    <h1>Hugo Link Checker Report</h1>
    <p>Generated: %s</p>
    
    <div class="summary">
        <h2>Summary</h2>
        <ul>
            <li>Files scanned: %d</li>
            <li>Total links: %d</li>
            <li>Unique links: %d</li>
            <li>Broken links: %d</li>
            <li>Internal links: %d</li>
            <li>External links: %d</li>
        </ul>
    </div>
`, time.Now().Format(time.RFC3339), summary.TotalFiles, summary.TotalLinks, 
   summary.UniqueLinks, summary.BrokenLinks, summary.InternalLinks, summary.ExternalLinks)
	
	for _, file := range files {
		fmt.Fprintf(writer, `    <div class="file">
        <h3>%s</h3>
        <p><strong>Canonical:</strong> %s</p>
        <p><strong>Links found:</strong> %d</p>
`, file.Path, file.CanonicalPath, len(file.Links))
		
		for _, link := range file.Links {
			status := "ok"
			statusText := "OK"
			if link.StatusCode >= 400 || (link.StatusCode == 0 && link.ErrorMessage != "") {
				status = "broken"
				statusText = "BROKEN"
				if link.ErrorMessage != "" {
					statusText = fmt.Sprintf("BROKEN (%s)", link.ErrorMessage)
				}
			}
			
			linkClass := "internal"
			if link.Type == scanner.LinkTypeExternal {
				linkClass = "external"
			}
			
			fmt.Fprintf(writer, `        <div class="link %s %s">%s [%s] - %s</div>
`, status, linkClass, link.URL, linkClass, statusText)
		}
		
		fmt.Fprintf(writer, "    </div>\n")
	}
	
	fmt.Fprintf(writer, `</body>
</html>`)
	
	return nil
}

func calculateSummary(files []*scanner.File) ReportSummary {
	summary := ReportSummary{
		TotalFiles: len(files),
	}
	
	uniqueURLs := make(map[string]bool)
	
	for _, file := range files {
		summary.TotalLinks += len(file.Links)
		
		for _, link := range file.Links {
			uniqueURLs[link.URL] = true
			
			if link.Type == scanner.LinkTypeExternal {
				summary.ExternalLinks++
			} else {
				summary.InternalLinks++
			}
			
			if link.StatusCode >= 400 || (link.StatusCode == 0 && link.ErrorMessage != "") {
				summary.BrokenLinks++
			}
		}
	}
	
	summary.UniqueLinks = len(uniqueURLs)
	return summary
}

// isMarkdownOrHTML checks if a file is a markdown or HTML file based on its extension
func isMarkdownOrHTML(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	return ext == ".md" || ext == ".markdown" || ext == ".html" || ext == ".htm"
}

func getUniqueLinks(files []*scanner.File) []UniqueLink {
	linkMap := make(map[string]*UniqueLink)
	
	for _, file := range files {
		for _, link := range file.Links {
			if existing, exists := linkMap[link.URL]; exists {
				existing.FoundInFiles = append(existing.FoundInFiles, file.Path)
			} else {
				linkType := "internal"
				if link.Type == scanner.LinkTypeExternal {
					linkType = "external"
				}
				
				linkMap[link.URL] = &UniqueLink{
					URL:          link.URL,
					Type:         linkType,
					StatusCode:   link.StatusCode,
					ErrorMessage: link.ErrorMessage,
					LastChecked:  link.LastChecked,
					FoundInFiles: []string{file.Path},
				}
			}
		}
	}
	
	result := make([]UniqueLink, 0, len(linkMap))
	for _, link := range linkMap {
		result = append(result, *link)
	}
	
	return result
}
