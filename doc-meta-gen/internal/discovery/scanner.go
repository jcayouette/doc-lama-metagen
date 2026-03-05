package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/scribe/doc-meta-gen/internal/providers"
)

// Scanner handles file discovery
type Scanner struct {
	rootDir   string
	providers []providers.ContentProvider
}

// NewScanner creates a new file scanner
func NewScanner(rootDir string, providers []providers.ContentProvider) *Scanner {
	return &Scanner{
		rootDir:   rootDir,
		providers: providers,
	}
}

// Scan walks the directory tree and finds processable files
func (s *Scanner) Scan() ([]string, error) {
	var files []string

	err := filepath.Walk(s.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if any provider can handle this file
		for _, provider := range s.providers {
			if provider.CanHandle(path) {
				files = append(files, path)
				break
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan directory: %w", err)
	}

	return files, nil
}

// FilterByType filters files based on file type
func FilterByType(files []string, fileType string) []string {
	if fileType == "all" {
		return files
	}

	var filtered []string
	for _, file := range files {
		ext := strings.ToLower(filepath.Ext(file))
		
		switch fileType {
		case "asciidoc":
			if ext == ".adoc" {
				filtered = append(filtered, file)
			}
		case "docbook":
			if ext == ".xml" {
				filtered = append(filtered, file)
			}
		}
	}

	return filtered
}
