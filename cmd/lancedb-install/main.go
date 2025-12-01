// Command lancedb-install downloads pre-built LanceDB static libraries from GitHub releases.
//
// Usage:
//
//	go run github.com/aqua777/go-lancedb/cmd/lancedb-install@latest
//	go run github.com/aqua777/go-lancedb/cmd/lancedb-install@latest --version v0.0.7
//
// The installer downloads the correct library for your OS/architecture to ~/.lancedb/libs/
// and outputs the CGO_LDFLAGS needed to build projects using go-lancedb.
package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	repoOwner     = "aqua777"
	repoName      = "go-lancedb"
	libraryName   = "liblancedb_cgo.a"
	latestVersion = "v0.0.7" // Updated when new releases are made
)

// Platform-specific linker flags (excluding the library path which we add dynamically)
var platformFlags = map[string]string{
	"darwin-arm64": "-lm -ldl -lresolv -framework CoreFoundation -framework Security -framework SystemConfiguration",
	"darwin-amd64": "-lm -ldl -lresolv -framework CoreFoundation -framework Security -framework SystemConfiguration",
	"linux-arm64":  "-lm -ldl -lpthread",
	"linux-amd64":  "-lm -ldl -lpthread",
}

func main() {
	version := flag.String("version", latestVersion, "Version to download (e.g., v0.0.7)")
	flag.Parse()

	goos := runtime.GOOS
	goarch := runtime.GOARCH
	platform := fmt.Sprintf("%s-%s", goos, goarch)

	// Validate platform
	if _, ok := platformFlags[platform]; !ok {
		fmt.Fprintf(os.Stderr, "Error: Unsupported platform: %s\n", platform)
		fmt.Fprintf(os.Stderr, "Supported platforms: darwin-arm64, darwin-amd64, linux-arm64, linux-amd64\n")
		os.Exit(1)
	}

	// Determine install directory using GOPATH
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		// Default GOPATH is ~/go
		homeDir, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Cannot determine home directory: %v\n", err)
			os.Exit(1)
		}
		gopath = filepath.Join(homeDir, "go")
	}

	libDir := filepath.Join(gopath, "lib", "lancedb", platform)
	libPath := filepath.Join(libDir, libraryName)

	// Create directory
	if err := os.MkdirAll(libDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Cannot create directory %s: %v\n", libDir, err)
		os.Exit(1)
	}

	// Construct download URL
	// Format: https://github.com/aqua777/go-lancedb/releases/download/v0.0.7/liblancedb_cgo-darwin-arm64.a
	downloadURL := fmt.Sprintf(
		"https://github.com/%s/%s/releases/download/%s/liblancedb_cgo-%s.a",
		repoOwner, repoName, *version, platform,
	)

	fmt.Printf("Downloading LanceDB library for %s...\n", platform)
	fmt.Printf("URL: %s\n", downloadURL)

	// Download the library
	if err := downloadFile(libPath, downloadURL); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Download failed: %v\n", err)
		fmt.Fprintf(os.Stderr, "\nPossible causes:\n")
		fmt.Fprintf(os.Stderr, "  - Version %s doesn't exist\n", *version)
		fmt.Fprintf(os.Stderr, "  - Binary for %s not available in this release\n", platform)
		fmt.Fprintf(os.Stderr, "  - Network connectivity issues\n")
		fmt.Fprintf(os.Stderr, "\nCheck available releases at:\n")
		fmt.Fprintf(os.Stderr, "  https://github.com/%s/%s/releases\n", repoOwner, repoName)
		os.Exit(1)
	}

	// Get file size for verification
	info, err := os.Stat(libPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Cannot stat downloaded file: %v\n", err)
		os.Exit(1)
	}

	// Sanity check - LFS pointer files are tiny (~130 bytes), real libraries are MB+
	if info.Size() < 10000 {
		fmt.Fprintf(os.Stderr, "Warning: Downloaded file is suspiciously small (%d bytes).\n", info.Size())
		fmt.Fprintf(os.Stderr, "This might be a Git LFS pointer file, not the actual library.\n")
		fmt.Fprintf(os.Stderr, "Please check the release assets at:\n")
		fmt.Fprintf(os.Stderr, "  https://github.com/%s/%s/releases/tag/%s\n", repoOwner, repoName, *version)
		os.Exit(1)
	}

	fmt.Printf("\nSuccess! Library installed to:\n")
	fmt.Printf("  %s (%d MB)\n", libPath, info.Size()/(1024*1024))

	// Output CGO_LDFLAGS
	flags := platformFlags[platform]
	cgoLdflags := fmt.Sprintf("-L%s -llancedb_cgo %s", libDir, flags)

	fmt.Printf("\n%s\n", strings.Repeat("=", 60))
	fmt.Printf("Add this to your shell profile (~/.bashrc, ~/.zshrc, etc.):\n")
	fmt.Printf("\n%s\n\n", strings.Repeat("=", 60))
	fmt.Printf("export CGO_LDFLAGS=\"%s\"\n", cgoLdflags)
	fmt.Printf("\n%s\n", strings.Repeat("=", 60))
	fmt.Printf("Or run this command to set it for the current session:\n")
	fmt.Printf("\n%s\n\n", strings.Repeat("=", 60))
	fmt.Printf("export CGO_LDFLAGS='%s'\n", cgoLdflags)
	fmt.Printf("\nThen build your project:\n")
	fmt.Printf("  go build ./...\n")
}

func downloadFile(filepath string, url string) error {
	// Create temporary file
	tmpPath := filepath + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("cannot create file: %w", err)
	}
	defer out.Close()

	// Download
	resp, err := http.Get(url)
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		os.Remove(tmpPath)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Copy with progress indication
	size, err := io.Copy(out, resp.Body)
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("download interrupted: %w", err)
	}

	out.Close()

	// Rename temp file to final destination
	if err := os.Rename(tmpPath, filepath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("cannot rename file: %w", err)
	}

	fmt.Printf("Downloaded %d bytes\n", size)
	return nil
}

