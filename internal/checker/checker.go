package checker

import "fmt"

// Check performs a placeholder site check for the given path.
// Real link-checking logic will live here later.
func Check(path string) error {
    fmt.Printf("Checking site at %s\n", path)
    return nil
}
