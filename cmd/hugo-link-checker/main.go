package main

import (
    "bufio"
    "flag"
    "fmt"
    "os"
    "regexp"
    "strings"

    "github.com/infodancer/hugo-link-checker/internal/checker"
    "github.com/infodancer/hugo-link-checker/internal/reporter"
    "github.com/infodancer/hugo-link-checker/internal/scanner"
    "github.com/infodancer/hugo-link-checker/internal/version"
)

func main() {
    var (
        showVersion   bool
        outputFile    string
        format        string
        noReport      bool
        rootDir       string
        checkImages   bool
        checkExternal bool
        checkPublic   bool
        baseURL       string
        verbose       bool
    )
    
    flag.BoolVar(&showVersion, "version", false, "Print version and exit")
    flag.StringVar(&outputFile, "output", "", "Output file for report (default: stdout)")
    flag.StringVar(&format, "format", "text", "Report format: text, json, html")
    flag.BoolVar(&noReport, "no-report", false, "Don't generate report, just return exit code based on broken links")
    flag.StringVar(&rootDir, "root", ".", "Root directory to scan")
    flag.BoolVar(&checkImages, "check-images", false, "Check image links (img src, markdown images)")
    flag.BoolVar(&checkExternal, "check-external", false, "Check external links (default: only check internal links)")
    flag.BoolVar(&checkPublic, "check-public", false, "Check for link destinations in Hugo's public directory")
    flag.StringVar(&baseURL, "base-url", "", "Base URL prefix to use when checking internal links online (e.g., https://example.com)")
    flag.BoolVar(&verbose, "verbose", false, "Verbose output: show all candidate paths checked for broken internal links")
    flag.Parse()

    if showVersion {
        fmt.Println("hugo-link-checker", version.Version)
        os.Exit(0)
    }

    // Validate format
    var reportFormat reporter.ReportFormat
    switch format {
    case "text":
        reportFormat = reporter.FormatText
    case "json":
        reportFormat = reporter.FormatJSON
    case "html":
        reportFormat = reporter.FormatHTML
    default:
        fmt.Fprintf(os.Stderr, "Invalid format: %s. Valid formats: text, json, html\n", format)
        os.Exit(1)
    }

    // Get paths to scan from command line arguments, or use root directory if none specified
    pathsToScan := flag.Args()
    if len(pathsToScan) == 0 {
        pathsToScan = []string{rootDir}
    }
    
    // Scan for files in specified paths
    files := make(map[string]*scanner.File)
    for _, path := range pathsToScan {
        pathFiles, err := scanner.EnumerateFiles(path, []string{".md", ".html", ".htm"})
        if err != nil {
            fmt.Fprintf(os.Stderr, "Error scanning files in %s: %v\n", path, err)
            os.Exit(1)
        }
        // Merge files from this path into the main files map
        for k, v := range pathFiles {
            files[k] = v
        }
    }
    
    fileList := scanner.GetFileList(files)
    
    // Load ignore patterns
    ignorePatterns, err := loadIgnorePatterns()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error loading ignore patterns: %v\n", err)
        os.Exit(1)
    }
    
    // Parse links from each file
    for _, file := range fileList {
        err := scanner.ParseLinksFromFile(file, checkImages)
        if err != nil {
            fmt.Fprintf(os.Stderr, "Error parsing links from %s: %v\n", file.Path, err)
            continue
        }
        
        // Apply ignore patterns
        applyIgnorePatterns(file, ignorePatterns)
        
        // Debug: Print ignored links if verbose
        if verbose {
            for _, link := range file.Links {
                if link.Ignored {
                    fmt.Fprintf(os.Stderr, "DEBUG: Ignored link: %s in file %s\n", link.URL, file.Path)
                }
            }
        }
    }
    
    // Check all links
    err = checker.CheckLinks(fileList, rootDir, checkExternal, checkPublic, baseURL, verbose)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error checking links: %v\n", err)
        os.Exit(1)
    }
    
    // Count broken links
    brokenCount := checker.CountBrokenLinks(fileList)
    
    if noReport {
        // Just exit with the number of broken links as exit code
        // Cap at 255 for valid exit codes
        if brokenCount > 255 {
            os.Exit(255)
        }
        os.Exit(brokenCount)
    }
    
    // Generate report
    reportOptions := reporter.ReportOptions{
        Format:     reportFormat,
        OutputFile: outputFile,
    }
    
    err = reporter.GenerateReport(fileList, reportOptions)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error generating report: %v\n", err)
        os.Exit(1)
    }
    
    // Exit with error code if broken links found
    if brokenCount > 0 {
        if brokenCount > 255 {
            os.Exit(255)
        }
        os.Exit(brokenCount)
    }
}

// loadIgnorePatterns reads the .hugo-link-checker-ignore file and returns compiled regex patterns
func loadIgnorePatterns() ([]*regexp.Regexp, error) {
    file, err := os.Open(".hugo-link-checker-ignore")
    if err != nil {
        if os.IsNotExist(err) {
            // Ignore file doesn't exist, return empty patterns
            return nil, nil
        }
        return nil, err
    }
    defer func() {
        if closeErr := file.Close(); closeErr != nil {
            fmt.Fprintf(os.Stderr, "Warning: failed to close ignore file: %v\n", closeErr)
        }
    }()
    
    var patterns []*regexp.Regexp
    scanner := bufio.NewScanner(file)
    
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        
        // Skip empty lines and comments (lines starting with #)
        if line == "" || strings.HasPrefix(line, "#") {
            continue
        }
        
        // Compile the regex pattern
        pattern, err := regexp.Compile(line)
        if err != nil {
            fmt.Fprintf(os.Stderr, "Warning: Invalid regex pattern '%s': %v\n", line, err)
            continue
        }
        
        patterns = append(patterns, pattern)
    }
    
    if err := scanner.Err(); err != nil {
        return nil, err
    }
    
    return patterns, nil
}

// applyIgnorePatterns marks links as ignored if they match any ignore pattern
func applyIgnorePatterns(file *scanner.File, patterns []*regexp.Regexp) {
    for i := range file.Links {
        link := &file.Links[i]
        
        // Check if this link matches any ignore pattern
        for _, pattern := range patterns {
            if pattern.MatchString(link.URL) {
                link.Ignored = true
                fmt.Fprintf(os.Stderr, "DEBUG: Ignoring link %s (matched pattern %s)\n", link.URL, pattern.String())
                break
            }
        }
    }
}
