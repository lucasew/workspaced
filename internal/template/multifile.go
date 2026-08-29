package template

import (
	"os"
	"strconv"
	"strings"
)

const (
	markerFileStart = "<<<WORKSPACED_FILE:"
	markerFileEnd   = "<<<WORKSPACED_ENDFILE>>>"
)

// MultiFile representa um arquivo em template multi-file
type MultiFile struct {
	Name    string
	Mode    os.FileMode
	Content string
}

// ParseMultiFile detecta e parseia templates multi-file
func ParseMultiFile(rendered []byte) ([]MultiFile, bool) {
	content := string(rendered)
	if !strings.Contains(content, markerFileStart) {
		return nil, false
	}

	var files []MultiFile
	parts := strings.Split(content, markerFileStart)

	for i, part := range parts {
		if i == 0 {
			// Skip content before first marker
			continue
		}

		// Parse header: filename:mode>>>
		before, after, ok := strings.Cut(part, ">>>")
		if !ok {
			continue
		}
		header := before
		rest := after

		// Split header
		headerParts := strings.SplitN(header, ":", 2)
		if len(headerParts) != 2 {
			continue
		}
		filename := headerParts[0]
		modeStr := headerParts[1]

		// Find end marker
		endIdx := strings.Index(rest, markerFileEnd)
		if endIdx == -1 {
			// No end marker, take rest of content
			endIdx = len(rest)
		}

		fileContent := strings.TrimSpace(rest[:endIdx])

		// Parse mode
		var mode os.FileMode = 0644
		if modeStr != "" {
			if parsed, err := strconv.ParseUint(modeStr, 8, 32); err == nil {
				mode = os.FileMode(parsed)
			}
		}

		files = append(files, MultiFile{
			Name:    filename,
			Mode:    mode,
			Content: fileContent,
		})
	}

	return files, len(files) > 0
}
