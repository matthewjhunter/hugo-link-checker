package main

import (
    "flag"
    "fmt"
    "os"

    "github.com/infodancer/hugo-link-checker/internal/scanner"
    "github.com/infodancer/hugo-link-checker/internal/version"
)

func main() {
    var showVersion bool
    flag.BoolVar(&showVersion, "version", false, "Print version and exit")
    flag.Parse()

    if showVersion {
        fmt.Println("hugo-link-checker", version.Version)
        os.Exit(0)
    }

    // Example usage of the file scanner
    files, err := scanner.EnumerateFiles(".", []string{".md", ".html", ".htm"})
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error scanning files: %v\n", err)
        os.Exit(1)
    }
    
    fmt.Printf("Found %d unique files\n", len(files))
    
    // Parse links from each file
    for _, file := range scanner.GetFileList(files) {
        err := scanner.ParseLinksFromFile(file)
        if err != nil {
            fmt.Fprintf(os.Stderr, "Error parsing links from %s: %v\n", file.Path, err)
            continue
        }
        
        fmt.Printf("File: %s (canonical: %s) - %d links found\n", 
            file.Path, file.CanonicalPath, len(file.Links))
        
        for _, link := range file.Links {
            linkType := "internal"
            if link.Type == scanner.LinkTypeExternal {
                linkType = "external"
            }
            fmt.Printf("  %s (%s)\n", link.URL, linkType)
        }
    }
}
