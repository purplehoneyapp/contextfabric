# ContextFabric 🧵

ContextFabric is a blazing-fast, single-binary CLI tool that spins up a local web UI to help you pack your codebase into Large Language Model (LLM) ready text files. 

Stop manually copying and pasting dozens of files into ChatGPT, Claude, or Gemini. With ContextFabric, you can visually navigate your project, select exactly the files you need, save workflow presets, and generate a perfectly formatted context file in seconds.

## ✨ Features

* **Interactive Local Web UI:** No clunky terminal checklists. ContextFabric serves an embedded, VS Code-style file tree directly to your browser.
* **Instant Search & Filter:** Quickly find files across massive monorepos with real-time text filtering.
* **Smart Ignoring:** Automatically respects your project's `.gitignore`, `.contextfabric_ignore`, and ignores standard noise like `.git/` and `node_modules/`.
* **Team Presets:** Save and load custom selections (e.g., "Auth Core", "Matching Engine") via a `.context-presets.yaml` file that can be committed to your repository.
* **Dual Output:** * Generates `<dir>-context.txt` containing the raw, formatted contents of all selected files.
  * Generates `<dir>-project-tree.txt` providing a visual ASCII map of your project architecture for the LLM.
* **Zero Dependencies:** Compiles to a single Go binary. No Node.js, no Python, no messy local environments.

## 🚀 Installation

Ensure you have Go installed on your machine, then clone this repository and build the binary:

```bash
git clone [https://github.com/yourusername/contextfabric.git](https://github.com/yourusername/contextfabric.git)
cd contextfabric
go mod tidy
go build -o contextfabric
go install
```

## 🛠️ Usage
Navigate to the root directory of the project you want to generate context for (e.g., your backend API or frontend monorepo), and run the tool:

```bash
cd /path/to/your/project
contextfabric
```

Your default web browser will instantly open to http://127.0.0.1:8080.
Browse the file tree, search for specific files, or load a saved Preset from the left sidebar.

Review your selection in the right sidebar.

Click Generate Context.

Find your newly generated *-context.txt and *-project-tree.txt files in your project root!

Press Ctrl+C in your terminal to safely shut down the ContextFabric server when you are done.

## ⚙️ Configuration
ContextFabric is designed to "just work," but you can customize its behavior using files placed in the root directory of the project you are scanning:

.contextfabric_ignore
Sometimes you have files that belong in Git, but you never want to send them to an LLM (e.g., massive database seed files, minified .js bundles, or static image assets). Create a .contextfabric_ignore file using standard Gitignore syntax:

Plaintext
# .contextfabric_ignore
data/seeds/*.yaml
public/assets/
docs/old-architecture/
.context-presets.yaml
Presets are saved here automatically when you use the "Save Selection" button in the UI. You can commit this file to your repository so your entire development team shares the same context groupings.

YAML
# .context-presets.yaml
"Auth Flow":
  - services/bigapi/internal/handler/rest/auth.go
  - services/bigapi/internal/processor/auth
"Matching Engine":
  - services/bigapi/internal/processor/matching/processor.go
  - services/bigapi/internal/service/match.go
🤝 Output Format
The generated <dir>-context.txt file uses a clean, markdown-friendly header format that helps LLMs understand file boundaries:

Plaintext
## internal/service/auth.go
package auth
// ... file contents ...

## internal/repository/user.go
package repository
// ... file contents ...