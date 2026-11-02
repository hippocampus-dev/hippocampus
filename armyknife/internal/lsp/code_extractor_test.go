package lsp_test

import (
	"armyknife/internal/lsp"
	"os"
	"path/filepath"
	"testing"
)

func TestGetCodeFromRange(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.go")
	content := `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}

func add(a int, b int) int {
	return a + b
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tests := []struct {
		name      string
		fileURI   string
		codeRange lsp.Range
		expected  string
	}{
		{
			name:    "Single line function",
			fileURI: "file://" + testFile,
			codeRange: lsp.Range{
				Start: lsp.Position{Line: 4, Character: 0},
				End:   lsp.Position{Line: 6, Character: 1},
			},
			expected: "func main() {\n\tfmt.Println(\"Hello, World!\")\n}",
		},
		{
			name:    "Multi-line function",
			fileURI: "file://" + testFile,
			codeRange: lsp.Range{
				Start: lsp.Position{Line: 8, Character: 0},
				End:   lsp.Position{Line: 10, Character: 1},
			},
			expected: "func add(a int, b int) int {\n\treturn a + b\n}",
		},
		{
			name:    "Partial line selection",
			fileURI: "file://" + testFile,
			codeRange: lsp.Range{
				Start: lsp.Position{Line: 5, Character: 1},
				End:   lsp.Position{Line: 5, Character: 29},
			},
			expected: "fmt.Println(\"Hello, World!\")",
		},
	}

	for _, tt := range tests {
		name := tt.name
		fileURI := tt.fileURI
		codeRange := tt.codeRange
		expected := tt.expected
		t.Run(name, func(t *testing.T) {
			code, err := lsp.GetCodeFromRange(fileURI, codeRange)
			if err != nil {
				t.Errorf("GetCodeFromRange failed: %v", err)
				return
			}
			if code != expected {
				t.Errorf("expected:\n%s\ngot:\n%s", expected, code)
			}
		})
	}
}
