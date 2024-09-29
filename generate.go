package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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
		return false
	}
	defer file.Close()

	var buf [512]byte
	n, err := file.Read(buf[:])
	if err != nil {
		return false
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

func generateProjectSummary(rootDir string) {

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

	outputFile := "output.txt"
	outFile, err := os.Create(outputFile)
	if err != nil {
		fmt.Printf("failed to create output file: %v", err)
		return
	}
	defer outFile.Close()

	_, err = fmt.Fprint(outFile, "## File Contents\n\n")
	if err != nil {
		fmt.Printf("failed to write to output file: %v", err)
		return
	}

	// path : full path
	// d : directory entry
	err = filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		fmt.Printf("Processing %s\n", path)
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %v", err)
		}

		if shouldIgnore(relPath, gitIgnore, summaryIgnore) {
			fmt.Printf("Ignoring %s", relPath)
			if d.IsDir() {
				fmt.Printf(" (directory)\n")
				return filepath.SkipDir
			}
			fmt.Printf(" (file)\n")
			return nil
		}

		if d.IsDir() {
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
	})

	if err != nil {
		fmt.Printf("failed to walk directory: %v", err)
		return
	}
}

func generate() {
	var projectDirectory string
	fmt.Print("Enter the project directory path (leave blank for current directory): ")
	fmt.Scanln(&projectDirectory)
	if projectDirectory == "" {
		var err error
		projectDirectory, err = os.Getwd()
		if err != nil {
			fmt.Println("Error getting current directory:", err)
			return
		}
	}
	generateProjectSummary(projectDirectory)
}
