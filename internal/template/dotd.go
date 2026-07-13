package template

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ProcessDotD processes a .d.tmpl directory (file concatenation).
func (e *Engine) ProcessDotD(ctx context.Context, dirPath string, data any) ([]byte, error) {
	if _, err := os.Stat(dirPath); errors.Is(err, os.ErrNotExist) {
		return nil, nil // Empty content if directory doesn't exist
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	var fileNames []string
	for _, entry := range entries {
		if !entry.IsDir() {
			fileNames = append(fileNames, entry.Name())
		}
	}

	var result bytes.Buffer
	for _, fileName := range fileNames {
		filePath := filepath.Join(dirPath, fileName)
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", fileName, err)
		}

		if strings.HasSuffix(fileName, ".tmpl") {
			rendered, err := e.Render(ctx, string(content), data)
			if err != nil {
				return nil, fmt.Errorf("failed to render template %s: %w", fileName, err)
			}
			result.Write(rendered)
		} else {
			result.Write(content)
		}

		result.WriteString("\n")
	}

	return result.Bytes(), nil
}
