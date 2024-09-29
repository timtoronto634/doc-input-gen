package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
)

var predefinedIgnores = []string{
	".git",
	".vscode/",
	"node_modules/",
	"vendor/",
	".idea/",
	".gitignore",
	".summaryignore",
	"output.txt",
}

func isBinary(filePath string) bool {
	file, err := os.Open(filePath)
	if err != nil {
		return true // Assume binary if we can't open the file
	}
	defer file.Close()

	var buf [512]byte
	n, err := file.Read(buf[:])
	if err != nil && err != io.EOF {
		return true // Assume binary if we can't read the file
	}

	for _, b := range buf[:n] {
		if b == 0 {
			return true
		}
	}
	return false
}

func shouldIgnore(path string, gitIgnore, summaryIgnore *ignore.GitIgnore) bool {
	// Check predefined ignores
	for _, ignorePath := range predefinedIgnores {
		if strings.HasPrefix(path, ignorePath) {
			return true
		}
	}

	// Check .gitignore and .summaryignore patterns
	return (gitIgnore != nil && gitIgnore.MatchesPath(path)) ||
		(summaryIgnore != nil && summaryIgnore.MatchesPath(path))
}

type DirectoryStructure struct {
	Directories []string `json:"directories"`
	Files       []string `json:"files"`
}

func captureDirectoryStructure(rootDir string, gitIgnore, summaryIgnore *ignore.GitIgnore) map[string]DirectoryStructure {
	structure := make(map[string]DirectoryStructure)

	filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			relPath = ""
		}

		if shouldIgnore(relPath, gitIgnore, summaryIgnore) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		dirPath := filepath.Dir(relPath)
		if dirPath == "." {
			dirPath = ""
		}

		entry := structure[dirPath]
		if info.IsDir() {
			entry.Directories = append(entry.Directories, filepath.Base(relPath))
		} else {
			entry.Files = append(entry.Files, filepath.Base(relPath))
		}
		structure[dirPath] = entry

		return nil
	})

	return structure
}

func generateProjectSummary(rootDir string, regexPatterns []*regexp.Regexp) {
	gitIgnore, err := ignore.CompileIgnoreFile(filepath.Join(rootDir, ".gitignore"))
	if err != nil && !os.IsNotExist(err) {
		fmt.Printf("failed to compile .gitignore: %v\n", err)
		return
	}

	summaryIgnore, err := ignore.CompileIgnoreFile(filepath.Join(rootDir, ".summaryignore"))
	if err != nil && !os.IsNotExist(err) {
		fmt.Printf("failed to compile .summaryignore: %v\n", err)
		return
	}

	outputFile := filepath.Join(rootDir, "tmp/output.txt")
	outFile, err := os.Create(outputFile)
	if err != nil {
		fmt.Printf("failed to create output file: %v", err)
		return
	}
	defer outFile.Close()

	// Capture and write project structure
	structure := captureDirectoryStructure(rootDir, gitIgnore, summaryIgnore)
	structureJSON, err := json.MarshalIndent(structure, "", "  ")
	if err != nil {
		fmt.Printf("failed to marshal project structure: %v", err)
		return
	}

	_, err = fmt.Fprintf(outFile, "## Project Structure\n```json\n%s\n```\n\n", structureJSON)
	if err != nil {
		fmt.Printf("failed to write project structure: %v", err)
		return
	}

	_, err = fmt.Fprint(outFile, "## File Contents\n\n")
	if err != nil {
		fmt.Printf("failed to write to output file: %v", err)
		return
	}

	processFile := func(path string) error {
		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %v", err)
		}

		if shouldIgnore(relPath, gitIgnore, summaryIgnore) {
			fmt.Printf("Ignoring %s\n", relPath)
			return nil
		}

		if isBinary(path) {
			fmt.Printf("Ignoring binary file %s\n", relPath)
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %v", path, err)
		}

		_, err = fmt.Fprintf(outFile, "### %s\n```\n%s\n```\n", relPath, content)
		if err != nil {
			return fmt.Errorf("failed to write to output file: %v", err)
		}

		return nil
	}

	err = filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %v", err)
		}

		if len(regexPatterns) == 0 {
			// Process all files if no patterns are specified
			return processFile(path)
		}

		for _, pattern := range regexPatterns {
			if pattern.MatchString(relPath) {
				return processFile(path)
			}
		}

		return nil
	})

	if err != nil {
		fmt.Printf("failed to process files: %v", err)
		return
	}
}

func readRegexPatternsFromFile(filePath string) ([]*regexp.Regexp, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	var patterns []*regexp.Regexp
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		pattern := strings.TrimSpace(scanner.Text())
		if pattern != "" {
			regex, err := regexp.Compile(pattern)
			if err != nil {
				return nil, fmt.Errorf("failed to compile regex pattern '%s': %v", pattern, err)
			}
			patterns = append(patterns, regex)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %v", err)
	}

	return patterns, nil
}

func generate() {
	var rootDir string
	fmt.Print("Enter the root directory path (leave blank for current directory): ")
	fmt.Scanln(&rootDir)
	if rootDir == "" {
		var err error
		rootDir, err = os.Getwd()
		if err != nil {
			fmt.Println("Error getting current directory:", err)
			return
		}
	}

	var option string
	fmt.Print("Enter 'all' to process all files, or provide a filepath for regex patterns: ")
	fmt.Scanln(&option)

	var regexPatterns []*regexp.Regexp
	var err error
	if option != "all" && option != "" {
		regexPatterns, err = readRegexPatternsFromFile(filepath.Join(rootDir, option))
		if err != nil {
			fmt.Printf("Error reading regex patterns: %v\n", err)
			return
		}
	}

	generateProjectSummary(rootDir, regexPatterns)
}
