package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseLinksFromMarkdownFile(t *testing.T) {
	// Create a temporary test file
	testContent := `# Test Markdown File

This is a test markdown file with various link formats.

## Standard Links
[Google](https://www.google.com)
[Internal Link](./internal-page.html)
[Another Internal](../docs/readme.md)

## Autolinks
<https://example.com>
<http://test.org>

## Reference Links
[Reference Link][ref1]
[Another Reference][ref2]

[ref1]: https://reference1.com
[ref2]: ./local-reference.html

## Mixed Content
Here's a [mixed link](https://mixed.example.com) in a sentence.
And an internal [relative link](./relative.md) too.
`

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "test*.md")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Logf("Warning: failed to remove temp file: %v", err)
		}
	}()

	if _, err := tmpFile.WriteString(testContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	// Create File struct
	file := &File{
		Path:          tmpFile.Name(),
		CanonicalPath: tmpFile.Name(),
		Links:         make([]Link, 0),
	}

	// Parse links
	err = ParseLinksFromFile(file, false)
	if err != nil {
		t.Fatalf("ParseLinksFromFile failed: %v", err)
	}

	// Expected links
	expectedLinks := map[string]LinkType{
		"https://www.google.com":     LinkTypeExternal,
		"./internal-page.html":       LinkTypeInternal,
		"../docs/readme.md":          LinkTypeInternal,
		"https://example.com":        LinkTypeExternal,
		"http://test.org":            LinkTypeExternal,
		"https://reference1.com":     LinkTypeExternal,
		"./local-reference.html":     LinkTypeInternal,
		"https://mixed.example.com":  LinkTypeExternal,
		"./relative.md":              LinkTypeInternal,
	}

	// Check number of links
	if len(file.Links) != len(expectedLinks) {
		t.Errorf("Expected %d links, got %d", len(expectedLinks), len(file.Links))
		for i, link := range file.Links {
			t.Logf("Link %d: %s (%v)", i, link.URL, link.Type)
		}
	}

	// Check each link
	foundLinks := make(map[string]LinkType)
	for _, link := range file.Links {
		foundLinks[link.URL] = link.Type
	}

	for expectedURL, expectedType := range expectedLinks {
		if foundType, exists := foundLinks[expectedURL]; !exists {
			t.Errorf("Expected link %s not found", expectedURL)
		} else if foundType != expectedType {
			t.Errorf("Link %s: expected type %v, got %v", expectedURL, expectedType, foundType)
		}
	}
}

func TestParseLinksFromHTMLFile(t *testing.T) {
	testContent := `<!DOCTYPE html>
<html>
<head>
    <title>Test HTML File</title>
    <link rel="stylesheet" href="./styles.css">
    <link rel="icon" href="https://example.com/favicon.ico">
</head>
<body>
    <h1>Test HTML</h1>
    <p>This is a test HTML file with various link formats.</p>
    
    <a href="https://www.example.com">External Link</a>
    <a href="./internal.html">Internal Link</a>
    <a href="../docs/index.html" title="Documentation">Docs Link</a>
    
    <div>
        <a href="mailto:test@example.com">Email Link</a>
        <a href="https://github.com/user/repo">GitHub</a>
    </div>
</body>
</html>`

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "test*.html")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Logf("Warning: failed to remove temp file: %v", err)
		}
	}()

	if _, err := tmpFile.WriteString(testContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	// Create File struct
	file := &File{
		Path:          tmpFile.Name(),
		CanonicalPath: tmpFile.Name(),
		Links:         make([]Link, 0),
	}

	// Parse links
	err = ParseLinksFromFile(file, false)
	if err != nil {
		t.Fatalf("ParseLinksFromFile failed: %v", err)
	}

	// Expected links
	expectedLinks := map[string]LinkType{
		"./styles.css":                LinkTypeInternal,
		"https://example.com/favicon.ico": LinkTypeExternal,
		"https://www.example.com":     LinkTypeExternal,
		"./internal.html":             LinkTypeInternal,
		"../docs/index.html":          LinkTypeInternal,
		"mailto:test@example.com":     LinkTypeExternal, // mailto has a scheme, so it's external
		"https://github.com/user/repo": LinkTypeExternal,
	}

	// Check number of links
	if len(file.Links) != len(expectedLinks) {
		t.Errorf("Expected %d links, got %d", len(expectedLinks), len(file.Links))
		for i, link := range file.Links {
			t.Logf("Link %d: %s (%v)", i, link.URL, link.Type)
		}
	}

	// Check each link
	foundLinks := make(map[string]LinkType)
	for _, link := range file.Links {
		foundLinks[link.URL] = link.Type
	}

	for expectedURL, expectedType := range expectedLinks {
		if foundType, exists := foundLinks[expectedURL]; !exists {
			t.Errorf("Expected link %s not found", expectedURL)
		} else if foundType != expectedType {
			t.Errorf("Link %s: expected type %v, got %v", expectedURL, expectedType, foundType)
		}
	}
}

func TestEnumerateFiles(t *testing.T) {
	// Create a temporary directory structure
	tmpDir, err := os.MkdirTemp("", "test_enumerate")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Logf("Warning: failed to remove temp dir: %v", err)
		}
	}()

	// Create test files
	testFiles := []string{
		"test1.md",
		"test2.html",
		"test3.htm",
		"test4.txt",      // Should be ignored
		".hidden.md",     // Should be ignored (dot file)
		"subdir/test5.md",
	}

	// Create subdirectory
	subdir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	// Create test files
	for _, filename := range testFiles {
		fullPath := filepath.Join(tmpDir, filename)
		dir := filepath.Dir(fullPath)
		if dir != tmpDir {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatalf("Failed to create dir %s: %v", dir, err)
			}
		}
		
		file, err := os.Create(fullPath)
		if err != nil {
			t.Fatalf("Failed to create file %s: %v", fullPath, err)
		}
		if _, err := file.WriteString("# Test content\n[link](http://example.com)"); err != nil {
			t.Fatalf("Failed to write to file %s: %v", fullPath, err)
		}
		if err := file.Close(); err != nil {
			t.Fatalf("Failed to close file %s: %v", fullPath, err)
		}
	}

	// Test enumeration
	files, err := EnumerateFiles(tmpDir, []string{".md", ".html", ".htm"})
	if err != nil {
		t.Fatalf("EnumerateFiles failed: %v", err)
	}

	// Should find 4 files (test1.md, test2.html, test3.htm, subdir/test5.md)
	// Should ignore test4.txt and .hidden.md
	expectedCount := 4
	if len(files) != expectedCount {
		t.Errorf("Expected %d files, got %d", expectedCount, len(files))
		for path := range files {
			t.Logf("Found file: %s", path)
		}
	}

	// Check that the right files were found
	fileList := GetFileList(files)
	foundPaths := make(map[string]bool)
	for _, file := range fileList {
		// Get relative path for comparison
		relPath, _ := filepath.Rel(tmpDir, file.Path)
		foundPaths[relPath] = true
	}

	expectedPaths := []string{"test1.md", "test2.html", "test3.htm", "subdir/test5.md"}
	for _, expectedPath := range expectedPaths {
		if !foundPaths[expectedPath] {
			t.Errorf("Expected to find file %s", expectedPath)
		}
	}

	// Check that ignored files were not found
	ignoredPaths := []string{"test4.txt", ".hidden.md"}
	for _, ignoredPath := range ignoredPaths {
		if foundPaths[ignoredPath] {
			t.Errorf("Should not have found ignored file %s", ignoredPath)
		}
	}
}

func TestLinkTypeDetection(t *testing.T) {
	testCases := []struct {
		url      string
		expected LinkType
	}{
		{"https://example.com", LinkTypeExternal},
		{"http://example.com", LinkTypeExternal},
		{"ftp://example.com", LinkTypeExternal},
		{"./relative.html", LinkTypeInternal},
		{"../parent.html", LinkTypeInternal},
		{"/absolute.html", LinkTypeInternal},
		{"page.html", LinkTypeInternal},
		{"mailto:test@example.com", LinkTypeExternal},
		{"#fragment", LinkTypeInternal},
		{"", LinkTypeInternal}, // Empty URL treated as internal
	}

	for _, tc := range testCases {
		link := NewLink(tc.url)
		if link.Type != tc.expected {
			t.Errorf("URL %s: expected type %v, got %v", tc.url, tc.expected, link.Type)
		}
	}
}
