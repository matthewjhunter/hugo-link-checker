package checker

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/infodancer/hugo-link-checker/internal/scanner"
)

func TestCheckLinks_HugoTemplateSyntax(t *testing.T) {
	// Create test files with Hugo template syntax
	files := []*scanner.File{
		{
			Path: "test1.md",
			Links: []scanner.Link{
				{URL: "{{.Site.BaseURL}}/about", Type: scanner.LinkTypeInternal},
				{URL: "https://example.com/{{.Params.slug}}", Type: scanner.LinkTypeExternal},
				{URL: "/normal-link", Type: scanner.LinkTypeInternal},
				{URL: "{{< ref \"other-page\" >}}", Type: scanner.LinkTypeInternal},
			},
		},
	}

	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "test_hugo_template")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Logf("Warning: failed to remove temp dir: %v", err)
		}
	}()

	err = CheckLinks(files, tmpDir, false, false, "", false)
	if err != nil {
		t.Fatalf("CheckLinks failed: %v", err)
	}

	// Verify Hugo template links are marked as OK
	for i, link := range files[0].Links {
		if strings.Contains(link.URL, "{{") || strings.Contains(link.URL, "}}") {
			if link.StatusCode != 200 {
				t.Errorf("Link %d (%s) should have status 200, got %d", i, link.URL, link.StatusCode)
			}
			if link.ErrorMessage != "" {
				t.Errorf("Link %d (%s) should have no error message, got %s", i, link.URL, link.ErrorMessage)
			}
			if link.LastChecked.IsZero() {
				t.Errorf("Link %d (%s) should have LastChecked set", i, link.URL)
			}
		}
	}
}

func TestCheckExternalLink(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			w.WriteHeader(http.StatusOK)
		case "/notfound":
			w.WriteHeader(http.StatusNotFound)
		case "/error":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	testCases := []struct {
		url            string
		expectedStatus int
		expectError    bool
	}{
		{server.URL + "/ok", 200, false},
		{server.URL + "/notfound", 404, false},
		{server.URL + "/error", 500, false},
	}

	for _, tc := range testCases {
		link := &scanner.Link{URL: tc.url}
		err := checkExternalLink(client, link)
		
		if tc.expectError && err == nil {
			t.Errorf("Expected error for URL %s, but got none", tc.url)
		}
		if !tc.expectError && err != nil {
			t.Errorf("Unexpected error for URL %s: %v", tc.url, err)
		}
		if link.StatusCode != tc.expectedStatus {
			t.Errorf("Expected status %d for URL %s, got %d", tc.expectedStatus, tc.url, link.StatusCode)
		}
	}
}

func TestCheckMailtoLink(t *testing.T) {
	testCases := []struct {
		url            string
		expectedStatus int
		expectError    bool
	}{
		{"mailto:test@example.com", 0, false}, // Will fail DNS lookup but shouldn't error
		{"mailto:invalid-email", 0, false},    // Invalid format
		{"mailto:", 0, false},                 // No email
	}

	for _, tc := range testCases {
		link := &scanner.Link{URL: tc.url}
		err := checkMailtoLink(link)
		
		if tc.expectError && err == nil {
			t.Errorf("Expected error for URL %s, but got none", tc.url)
		}
		if !tc.expectError && err != nil {
			t.Errorf("Unexpected error for URL %s: %v", tc.url, err)
		}
	}
}

func TestCheckInternalLink_LocalFiles(t *testing.T) {
	// Create a temporary directory structure
	tmpDir, err := os.MkdirTemp("", "test_internal_links")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Logf("Warning: failed to remove temp dir: %v", err)
		}
	}()

	// Create test files
	contentDir := filepath.Join(tmpDir, "content")
	staticDir := filepath.Join(tmpDir, "static")
	if err := os.MkdirAll(contentDir, 0755); err != nil {
		t.Fatalf("Failed to create content directory: %v", err)
	}
	if err := os.MkdirAll(staticDir, 0755); err != nil {
		t.Fatalf("Failed to create static directory: %v", err)
	}

	// Create some test files
	testFiles := []string{
		filepath.Join(contentDir, "about.md"),
		filepath.Join(contentDir, "posts", "index.md"),
		filepath.Join(staticDir, "image.png"),
	}

	for _, file := range testFiles {
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			t.Fatalf("Failed to create directory for %s: %v", file, err)
		}
		f, err := os.Create(file)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", file, err)
		}
		if err := f.Close(); err != nil {
			t.Fatalf("Failed to close test file %s: %v", file, err)
		}
	}

	client := &http.Client{Timeout: 5 * time.Second}

	testCases := []struct {
		url            string
		expectedStatus int
		description    string
	}{
		{"/about/", 200, "Hugo-style URL to content file"},
		{"/posts/", 200, "Hugo-style URL to index file"},
		{"/image.png", 200, "Static file"},
		{"/nonexistent/", 404, "Non-existent file"},
		{"#fragment", 200, "Fragment-only link"},
		{"/about/?param=value", 200, "URL with query parameters"},
	}

	for _, tc := range testCases {
		link := &scanner.Link{URL: tc.url, Type: scanner.LinkTypeInternal}
		err := checkInternalLink(link, tmpDir, false, "", client, false)
		if err != nil {
			t.Errorf("Unexpected error checking %s: %v", tc.url, err)
			continue
		}
		
		if link.StatusCode != tc.expectedStatus {
			t.Errorf("%s: expected status %d, got %d", tc.description, tc.expectedStatus, link.StatusCode)
		}
	}
}

func TestCheckInternalLink_WithBaseURL(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/about/":
			w.WriteHeader(http.StatusOK)
		case "/posts/":
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	testCases := []struct {
		url            string
		expectedStatus int
		description    string
	}{
		{"/about/", 200, "Existing page"},
		{"/posts/", 200, "Another existing page"},
		{"/nonexistent/", 404, "Non-existent page"},
	}

	for _, tc := range testCases {
		link := &scanner.Link{URL: tc.url, Type: scanner.LinkTypeInternal}
		err := checkInternalLink(link, "", false, server.URL, client, false)
		if err != nil {
			t.Errorf("Unexpected error checking %s: %v", tc.url, err)
			continue
		}
		
		if link.StatusCode != tc.expectedStatus {
			t.Errorf("%s: expected status %d, got %d", tc.description, tc.expectedStatus, link.StatusCode)
		}
	}
}

func TestCheckHugoFile(t *testing.T) {
	// Create a temporary Hugo site structure
	tmpDir, err := os.MkdirTemp("", "test_hugo_file")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Logf("Warning: failed to remove temp dir: %v", err)
		}
	}()

	// Create Hugo directory structure
	contentDir := filepath.Join(tmpDir, "content")
	staticDir := filepath.Join(tmpDir, "static")
	if err := os.MkdirAll(contentDir, 0755); err != nil {
		t.Fatalf("Failed to create content directory: %v", err)
	}
	if err := os.MkdirAll(staticDir, 0755); err != nil {
		t.Fatalf("Failed to create static directory: %v", err)
	}

	// Create test files
	testFiles := []string{
		filepath.Join(contentDir, "about.md"),
		filepath.Join(contentDir, "posts", "index.md"),
		filepath.Join(contentDir, "posts", "_index.md"),
		filepath.Join(staticDir, "images", "logo.png"),
	}

	for _, file := range testFiles {
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			t.Fatalf("Failed to create directory for %s: %v", file, err)
		}
		f, err := os.Create(file)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", file, err)
		}
		if err := f.Close(); err != nil {
			t.Fatalf("Failed to close test file %s: %v", file, err)
		}
	}

	testCases := []struct {
		linkPath string
		expected bool
		description string
	}{
		{"about/", true, "Hugo URL to content file"},
		{"posts/", true, "Hugo URL to index file"},
		{"images/logo.png", true, "Static file"},
		{"nonexistent/", false, "Non-existent file"},
		{"about", true, "Hugo URL without trailing slash"},
		{"posts", true, "Hugo URL without trailing slash to index"},
	}

	for _, tc := range testCases {
		result, _ := checkHugoFile(tc.linkPath, tmpDir, false)
		if result != tc.expected {
			t.Errorf("%s: expected %v, got %v", tc.description, tc.expected, result)
		}
	}
}

func TestCheckHugoFileVerbose(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "test_hugo_file_verbose")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Logf("Warning: failed to remove temp dir: %v", err)
		}
	}()

	// Test verbose mode returns checked paths
	found, checkedPaths := checkHugoFile("nonexistent/", tmpDir, true)
	
	if found {
		t.Error("Expected file not to be found")
	}
	
	if len(checkedPaths) == 0 {
		t.Error("Expected some checked paths to be returned in verbose mode")
	}

	// Test non-verbose mode doesn't return paths
	found, checkedPaths = checkHugoFile("nonexistent/", tmpDir, false)
	
	if found {
		t.Error("Expected file not to be found")
	}
	
	if len(checkedPaths) != 0 {
		t.Error("Expected no checked paths to be returned in non-verbose mode")
	}
}

func TestCountBrokenLinks(t *testing.T) {
	files := []*scanner.File{
		{
			Path: "test1.md",
			Links: []scanner.Link{
				{URL: "http://example.com", StatusCode: 200},
				{URL: "http://broken.com", StatusCode: 404},
				{URL: "http://error.com", StatusCode: 500},
				{URL: "http://timeout.com", StatusCode: 0, ErrorMessage: "timeout"},
			},
		},
		{
			Path: "test2.md",
			Links: []scanner.Link{
				{URL: "http://ok.com", StatusCode: 200},
				{URL: "http://another-broken.com", StatusCode: 403},
			},
		},
	}

	count := CountBrokenLinks(files)
	expected := 4 // 404, 500, timeout error, and 403
	
	if count != expected {
		t.Errorf("Expected %d broken links, got %d", expected, count)
	}
}

func TestCheckLinks_Integration(t *testing.T) {
	// Create a temporary directory structure
	tmpDir, err := os.MkdirTemp("", "test_check_links_integration")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Logf("Warning: failed to remove temp dir: %v", err)
		}
	}()

	// Create Hugo directory structure
	contentDir := filepath.Join(tmpDir, "content")
	if err := os.MkdirAll(contentDir, 0755); err != nil {
		t.Fatalf("Failed to create content directory: %v", err)
	}

	// Create a test file
	testFile := filepath.Join(contentDir, "about.md")
	f, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Failed to close test file: %v", err)
	}

	// Create test files with various link types
	files := []*scanner.File{
		{
			Path: "test.md",
			Links: []scanner.Link{
				{URL: "{{.Site.BaseURL}}/template", Type: scanner.LinkTypeInternal}, // Hugo template
				{URL: "/about/", Type: scanner.LinkTypeInternal},                   // Valid internal
				{URL: "/nonexistent/", Type: scanner.LinkTypeInternal},             // Invalid internal
				{URL: "#fragment", Type: scanner.LinkTypeInternal},                 // Fragment only
			},
		},
	}

	err = CheckLinks(files, tmpDir, false, false, "", false)
	if err != nil {
		t.Fatalf("CheckLinks failed: %v", err)
	}

	// Verify results
	expectedResults := []struct {
		index          int
		expectedStatus int
		description    string
	}{
		{0, 200, "Hugo template link should be OK"},
		{1, 200, "Valid internal link should be OK"},
		{2, 404, "Invalid internal link should be 404"},
		{3, 200, "Fragment-only link should be OK"},
	}

	for _, expected := range expectedResults {
		link := files[0].Links[expected.index]
		if link.StatusCode != expected.expectedStatus {
			t.Errorf("%s: expected status %d, got %d", expected.description, expected.expectedStatus, link.StatusCode)
		}
		if link.LastChecked.IsZero() {
			t.Errorf("Link %d should have LastChecked set", expected.index)
		}
	}
}
