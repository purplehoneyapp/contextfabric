package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/pkg/browser"
	ignore "github.com/sabhiram/go-gitignore"
	"gopkg.in/yaml.v3"
)

//go:embed index.html
var indexHTML []byte

type FileNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	IsDir    bool        `json:"isDir"`
	Children []*FileNode `json:"children,omitempty"`
}

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()

	// 1. Serve the Embedded Frontend UI
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write(indexHTML)
	})

	// 2. API Endpoint: Return the project tree structure
	mux.HandleFunc("/api/tree", func(w http.ResponseWriter, r *http.Request) {
		tree, err := buildDirectoryTree(cwd)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tree)
	})

	// 3. API Endpoint: Manage Presets
	mux.HandleFunc("/api/presets", func(w http.ResponseWriter, r *http.Request) {
		presetFile := filepath.Join(cwd, ".context-presets.yaml")

		if r.Method == http.MethodGet {
			presets := make(map[string][]string)
			data, err := os.ReadFile(presetFile)
			if err == nil {
				_ = yaml.Unmarshal(data, &presets)
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(presets)
			return
		}

		if r.Method == http.MethodPost {
			var presets map[string][]string
			if err := json.NewDecoder(r.Body).Decode(&presets); err != nil {
				http.Error(w, "Invalid JSON body", http.StatusBadRequest)
				return
			}

			yamlData, err := yaml.Marshal(presets)
			if err != nil {
				http.Error(w, "Failed to encode YAML", http.StatusInternalServerError)
				return
			}

			if err := os.WriteFile(presetFile, yamlData, 0644); err != nil {
				http.Error(w, "Failed to write preset file", http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusOK)
			return
		}

		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	// 4. API Endpoint: Generate the context file (NO SHUTDOWN)
	mux.HandleFunc("/api/generate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			SelectedPaths []string `json:"selectedPaths"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		// Generate the main context text file
		err := generateContextFile(cwd, req.SelectedPaths)
		if err != nil {
			http.Error(w, fmt.Sprintf("Context generation failed: %v", err), http.StatusInternalServerError)
			return
		}

		// Re-build the tree to ensure we have the latest state, then write the tree file
		treeRoot, err := buildDirectoryTree(cwd)
		if err == nil {
			if treeErr := generateTreeFile(cwd, treeRoot); treeErr != nil {
				fmt.Printf("Warning: Failed to generate project tree file: %v\n", treeErr)
			}
		} else {
			fmt.Printf("Warning: Failed to scan directory for tree file: %v\n", err)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	})

	srv := &http.Server{
		Addr:    "127.0.0.1:8080",
		Handler: mux,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Server error: %v\n", err)
		}
	}()

	fmt.Println("ContextFabric running on http://127.0.0.1:8080")
	fmt.Println("Press Ctrl+C in this terminal to stop the server.")
	_ = browser.OpenURL("http://127.0.0.1:8080")

	// Wait for OS interrupt signal (Ctrl+C)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\nShutting down ContextFabric server...")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

// buildDirectoryTree scans the directory and builds a nested tree.
func buildDirectoryTree(root string) (*FileNode, error) {
	var ignoreLines []string
	ignoreLines = append(ignoreLines, ".git", "node_modules", "vendor", "*-context.txt", "*-project-tree.txt")

	appendIgnoreFile := func(filename string) {
		content, err := os.ReadFile(filepath.Join(root, filename))
		if err == nil {
			lines := strings.Split(string(content), "\n")
			ignoreLines = append(ignoreLines, lines...)
		}
	}

	appendIgnoreFile(".gitignore")
	appendIgnoreFile(".contextfabric_ignore")

	ignorer := ignore.CompileIgnoreLines(ignoreLines...)
	rootNode := &FileNode{Name: filepath.Base(root), Path: ".", IsDir: true}
	nodeMap := make(map[string]*FileNode)
	nodeMap["."] = rootNode

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil || relPath == "." {
			return err
		}
		relPath = filepath.ToSlash(relPath)

		if ignorer != nil && ignorer.MatchesPath(relPath) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		node := &FileNode{Name: d.Name(), Path: relPath, IsDir: d.IsDir()}
		parentPath := filepath.ToSlash(filepath.Dir(relPath))

		if parentNode, exists := nodeMap[parentPath]; exists {
			parentNode.Children = append(parentNode.Children, node)
		}

		if d.IsDir() {
			nodeMap[relPath] = node
		}
		return nil
	})

	return rootNode, err
}

// generateContextFile takes the selected paths and writes the output file.
func generateContextFile(cwd string, selectedPaths []string) error {
	dirName := filepath.Base(cwd)
	outFileName := fmt.Sprintf("%s-context.txt", dirName)

	outFile, err := os.Create(outFileName)
	if err != nil {
		return err
	}
	defer outFile.Close()

	for _, relPath := range selectedPaths {
		absPath := filepath.Join(cwd, relPath)
		stat, err := os.Stat(absPath)
		if err != nil || stat.IsDir() {
			continue
		}

		content, err := os.ReadFile(absPath)
		if err != nil {
			fmt.Printf("Warning: Failed to read %s\n", relPath)
			continue
		}

		fmt.Fprintf(outFile, "## %s\n", relPath)
		outFile.Write(content)
		outFile.WriteString("\n")
	}

	return nil
}

// generateTreeFile creates a visual ASCII representation of the project hierarchy.
func generateTreeFile(cwd string, rootNode *FileNode) error {
	dirName := filepath.Base(cwd)
	outFileName := fmt.Sprintf("%s-project-tree.txt", dirName)

	outFile, err := os.Create(outFileName)
	if err != nil {
		return err
	}
	defer outFile.Close()

	var sb strings.Builder
	sb.WriteString(rootNode.Name + "\n")

	for i, child := range rootNode.Children {
		isLast := i == len(rootNode.Children)-1
		buildTreeString(child, "", isLast, &sb)
	}

	_, err = outFile.WriteString(sb.String())
	return err
}

// buildTreeString recursively constructs the ASCII tree branches.
func buildTreeString(node *FileNode, prefix string, isLast bool, sb *strings.Builder) {
	if isLast {
		sb.WriteString(prefix + "└── " + node.Name + "\n")
	} else {
		sb.WriteString(prefix + "├── " + node.Name + "\n")
	}

	newPrefix := prefix
	if isLast {
		newPrefix += "    "
	} else {
		newPrefix += "│   "
	}

	for i, child := range node.Children {
		childIsLast := i == len(node.Children)-1
		buildTreeString(child, newPrefix, childIsLast, sb)
	}
}
