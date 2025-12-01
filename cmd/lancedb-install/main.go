// Command lancedb-install downloads pre-built LanceDB static libraries from GitHub releases
// and generates a pkg-config file for use with CGO.
//
// Usage:
//
//	go run github.com/aqua777/go-lancedb/cmd/lancedb-install@latest
//	go run github.com/aqua777/go-lancedb/cmd/lancedb-install@latest --version v0.0.7
//
// The installer:
// 1. Downloads the correct library for your OS/architecture to $GOPATH/lib/lancedb/{os}-{arch}/
// 2. Generates a lancedb.pc file in $GOPATH/lib/pkgconfig/
// 3. Outputs instructions to set PKG_CONFIG_PATH
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

	// Library Install Path
	libDir := filepath.Join(gopath, "lib", "lancedb", platform)
	libPath := filepath.Join(libDir, libraryName)

	// PkgConfig Install Path
	pkgConfigDir := filepath.Join(gopath, "lib", "pkgconfig")
	pkgConfigPath := filepath.Join(pkgConfigDir, "lancedb.pc")

	// Create directories
	if err := os.MkdirAll(libDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Cannot create directory %s: %v\n", libDir, err)
		os.Exit(1)
	}
	if err := os.MkdirAll(pkgConfigDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Cannot create directory %s: %v\n", pkgConfigDir, err)
		os.Exit(1)
	}

	// Construct download URL
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

	// Verify download
	info, err := os.Stat(libPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Cannot stat downloaded file: %v\n", err)
		os.Exit(1)
	}

	if info.Size() < 10000 {
		fmt.Fprintf(os.Stderr, "Warning: Downloaded file is suspiciously small (%d bytes).\n", info.Size())
		fmt.Fprintf(os.Stderr, "This might be a Git LFS pointer file, not the actual library.\n")
		fmt.Fprintf(os.Stderr, "Please check the release assets at:\n")
		fmt.Fprintf(os.Stderr, "  https://github.com/%s/%s/releases/tag/%s\n", repoOwner, repoName, *version)
		os.Exit(1)
	}

	fmt.Printf("Library installed to: %s (%d MB)\n", libPath, info.Size()/(1024*1024))

	// Generate pkg-config file
	flags := platformFlags[platform]
	pcContent := fmt.Sprintf(`Name: lancedb
Description: LanceDB Static Library
Version: %s
Libs: -L%s -llancedb_cgo %s
Cflags: 
`, *version, libDir, flags)

	if err := os.WriteFile(pkgConfigPath, []byte(pcContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Cannot write pkg-config file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Pkg-config file created at: %s\n", pkgConfigPath)

	fmt.Printf("\n%s\n", strings.Repeat("=", 60))
	fmt.Printf("Setup Instructions:\n")
	fmt.Printf("%s\n\n", strings.Repeat("=", 60))
	fmt.Printf("1. Add this to your shell profile (~/.bashrc, ~/.zshrc, etc.):\n\n")
	fmt.Printf("   export PKG_CONFIG_PATH=\"$PKG_CONFIG_PATH:%s\"\n\n", pkgConfigDir)
	fmt.Printf("2. Apply changes (or open a new terminal):\n\n")
	fmt.Printf("   source ~/.zshrc  # or ~/.bashrc\n\n")
	fmt.Printf("3. Build your project:\n\n")
	fmt.Printf("   go build ./...\n")
	fmt.Printf("\n%s\n", strings.Repeat("=", 60))
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
	_, err = io.Copy(out, resp.Body)
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

	return nil
}
