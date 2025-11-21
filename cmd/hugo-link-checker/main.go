package main

import (
    "flag"
    "fmt"
    "os"

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

    fmt.Println("hugo-link-checker: no command specified. Use -version to print version.")
}
