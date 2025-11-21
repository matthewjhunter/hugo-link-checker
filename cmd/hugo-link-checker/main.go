package main

import (
    "flag"
    "fmt"
    "os"

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
    
    // Parse links from each file
    for _, file := range fileList {
        err := scanner.ParseLinksFromFile(file, checkImages)
        if err != nil {
            fmt.Fprintf(os.Stderr, "Error parsing links from %s: %v\n", file.Path, err)
            continue
        }
    }
    
    // Check all links
    err := checker.CheckLinks(fileList, rootDir, checkExternal, baseURL, verbose)
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
