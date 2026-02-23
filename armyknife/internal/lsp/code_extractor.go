package lsp

import (
	"bufio"
	"net/url"
	"os"
	"strings"

	"golang.org/x/xerrors"
)

func GetCodeFromRange(fileURI string, r Range) (string, error) {
	parsedURI, err := url.Parse(fileURI)
	if err != nil {
		return "", xerrors.Errorf("failed to parse URI: %w", err)
	}

	filePath := parsedURI.Path
	if parsedURI.Scheme == "file" && parsedURI.Host != "" {
		filePath = parsedURI.Host + parsedURI.Path
	}
	file, err := os.Open(filePath)
	if err != nil {
		return "", xerrors.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	var codeLines []string
	inRange := false

	for scanner.Scan() {
		line := scanner.Text()

		if lineNumber == r.Start.Line {
			inRange = true
			if r.Start.Character > 0 && r.Start.Character < len(line) {
				line = line[r.Start.Character:]
			}
		}

		if inRange {
			if lineNumber == r.End.Line {
				if r.End.Character < len(line) {
					if r.Start.Line == r.End.Line {
						line = line[:r.End.Character-r.Start.Character]
					} else {
						line = line[:r.End.Character]
					}
				}
				codeLines = append(codeLines, line)
				break
			}
			codeLines = append(codeLines, line)
		}

		lineNumber++
		if lineNumber > r.End.Line {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return "", xerrors.Errorf("failed to read file: %w", err)
	}

	return strings.Join(codeLines, "\n"), nil
}
