package main

import (
	"bufio"
	"fmt"
	"io"
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

func generateProjectSummary(rootDir string, targetFiles []string) {
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

	if len(targetFiles) == 0 {
		// Process all files
		err = filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d.IsDir() {
				return nil
			}

			return processFile(path)
		})
	} else {
		// Process only specified files
		for _, file := range targetFiles {
			fullPath := filepath.Join(rootDir, file)
			err := processFile(fullPath)
			if err != nil {
				fmt.Printf("Error processing file %s: %v\n", file, err)
			}
		}
	}

	if err != nil {
		fmt.Printf("failed to process files: %v", err)
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

	var option string
	fmt.Print("Enter 'all' to process all files, or provide a filepath for target files: ")
	fmt.Scanln(&option)

	var targetFiles []string
	if option != "all" && option != "" {
		// Read target files from the specified file
		file, err := os.Open(option)
		if err != nil {
			fmt.Printf("Error opening file %s: %v\n", option, err)
			return
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			targetFiles = append(targetFiles, strings.TrimSpace(scanner.Text()))
		}

		if err := scanner.Err(); err != nil {
			fmt.Printf("Error reading file %s: %v\n", option, err)
			return
		}
	}

	generateProjectSummary(projectDirectory, targetFiles)
}
