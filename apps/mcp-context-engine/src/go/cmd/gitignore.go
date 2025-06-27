package cmd

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// GitIgnore handles .gitignore pattern matching
type GitIgnore struct {
	patterns []string
	root     string
}

// NewGitIgnore creates a new GitIgnore instance
func NewGitIgnore(root string) *GitIgnore {
	gi := &GitIgnore{
		root: root,
		patterns: []string{
			// Always ignore these
			".git/",
			".git",
			"node_modules/",
			"node_modules",
			".DS_Store",
			"*.log",
			"dist/",
			"build/",
			"bin/",
			"coverage/",
			".cache/",
			"*.map",
			"*.min.js",
			"*.min.css",
			"vendor/",
			"__pycache__/",
			"*.pyc",
			".env",
			".env.local",
		},
	}
	
	// Load .gitignore from root
	gi.loadGitIgnore(filepath.Join(root, ".gitignore"))
	
	return gi
}

// loadGitIgnore reads patterns from a .gitignore file
func (gi *GitIgnore) loadGitIgnore(path string) {
	file, err := os.Open(path)
	if err != nil {
		return // .gitignore might not exist
	}
	defer file.Close()
	
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		gi.patterns = append(gi.patterns, line)
	}
}

// ShouldIgnore checks if a path should be ignored
func (gi *GitIgnore) ShouldIgnore(path string) bool {
	// Convert to relative path from root
	relPath, err := filepath.Rel(gi.root, path)
	if err != nil {
		return false
	}
	
	// Check each pattern
	for _, pattern := range gi.patterns {
		if gi.matchPattern(relPath, pattern) {
			return true
		}
	}
	
	return false
}

// matchPattern checks if a path matches a gitignore pattern
func (gi *GitIgnore) matchPattern(path, pattern string) bool {
	// Normalize separators
	path = filepath.ToSlash(path)
	pattern = filepath.ToSlash(pattern)
	
	// Directory pattern (ends with /)
	if strings.HasSuffix(pattern, "/") {
		dirPattern := strings.TrimSuffix(pattern, "/")
		// Check if any part of the path matches
		parts := strings.Split(path, "/")
		for _, part := range parts {
			if matched, _ := filepath.Match(dirPattern, part); matched {
				return true
			}
		}
		// Also check if path starts with pattern
		if strings.HasPrefix(path, dirPattern+"/") {
			return true
		}
	}
	
	// File or pattern matching
	if strings.Contains(pattern, "/") {
		// Path pattern - match from root
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
	} else {
		// Simple pattern - match basename
		base := filepath.Base(path)
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
		// Also check if any directory in path matches
		parts := strings.Split(path, "/")
		for _, part := range parts {
			if matched, _ := filepath.Match(pattern, part); matched {
				return true
			}
		}
	}
	
	return false
}

// WalkCodeFiles walks a directory tree, returning only code files that aren't ignored
func WalkCodeFiles(root string, isCodeFile func(string) bool) ([]string, error) {
	gi := NewGitIgnore(root)
	var files []string
	
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		
		// Check if should ignore
		if gi.ShouldIgnore(path) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		
		// Add code files
		if !info.IsDir() && isCodeFile(path) {
			files = append(files, path)
		}
		
		// Load nested .gitignore files
		if info.IsDir() {
			nestedGitIgnore := filepath.Join(path, ".gitignore")
			if _, err := os.Stat(nestedGitIgnore); err == nil {
				gi.loadGitIgnore(nestedGitIgnore)
			}
		}
		
		return nil
	})
	
	return files, err
}