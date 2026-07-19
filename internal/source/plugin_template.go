package source

import (
	"context"
	"errors"
	"fmt"
	"github.com/lucasew/workspaced/internal/configcue"
	"github.com/lucasew/workspaced/internal/template"
	"io"
	"path/filepath"
	"strings"
)

// TemplateExpanderPlugin renders templates and expands multi-file output.
type TemplateExpanderPlugin struct {
	engine *template.Engine
	data   any
}

// NewTemplateExpanderPlugin creates a template expansion plugin.
func NewTemplateExpanderPlugin(engine *template.Engine, data any) *TemplateExpanderPlugin {
	return &TemplateExpanderPlugin{
		engine: engine,
		data:   data,
	}
}

func (p *TemplateExpanderPlugin) Name() string {
	return "template-expander"
}

func (p *TemplateExpanderPlugin) Process(ctx context.Context, files []File) ([]File, error) {
	result := []File{}
	globalCfg, _ := p.data.(*configcue.Config)

	for _, f := range files {
		// Detect if the file is a template
		filename := filepath.Base(f.RelPath())
		parts := strings.Split(filename, ".")
		isTemplate := (len(parts) >= 2 && parts[len(parts)-1] == "tmpl") ||
			(len(parts) >= 3 && parts[len(parts)-2] == "tmpl")

		if !isTemplate {
			// Not a template, pass through
			result = append(result, f)
			continue
		}

		relPath := f.RelPath()
		if parts[len(parts)-1] == "tmpl" {
			// file.tmpl → file
			relPath = strings.TrimSuffix(relPath, ".tmpl")
		} else {
			// file.tmpl.ext → file.ext
			relPath = strings.TrimSuffix(relPath, ".tmpl"+filepath.Ext(relPath)) + filepath.Ext(relPath)
		}

		// Eagerly render to check if it's multi-file
		reader, err := f.Reader()
		if err != nil {
			return nil, fmt.Errorf("failed to read template source %s: %w", f.SourceInfo(), err)
		}
		srcContent, err := io.ReadAll(reader)
		if closeErr := reader.Close(); closeErr != nil {
			return nil, fmt.Errorf("failed to close template source %s: %w", f.SourceInfo(), closeErr)
		}
		if err != nil {
			return nil, err
		}

		templateData, err := buildTemplateData(ctx, globalCfg, f)
		if err != nil {
			return nil, err
		}

		rendered, err := p.engine.Render(ctx, string(srcContent), templateData)
		if err != nil {
			if errors.Is(err, template.ErrFileSkipped) {
				continue
			}
			return nil, fmt.Errorf("failed to render template %s: %w", f.SourceInfo(), err)
		}

		multiFiles, isMulti := template.ParseMultiFile(rendered)

		if isMulti {
			// One template → N files (EAGER)
			baseRelDir := filepath.Dir(relPath)
			baseName := strings.TrimSuffix(filepath.Base(relPath), filepath.Ext(relPath))

			if baseName != "_index" {
				baseRelDir = filepath.Join(baseRelDir, baseName)
			}

			for _, mf := range multiFiles {
				mfRelPath := filepath.Join(baseRelDir, mf.Name)
				result = append(result, &BufferFile{
					BasicFile: BasicFile{
						RelPathStr:    mfRelPath,
						TargetBaseDir: f.TargetBase(),
						FileMode:      mf.Mode,
						Info:          fmt.Sprintf("%s (multi:%s)", f.SourceInfo(), mf.Name),
						FileType:      TypeMultiFile,
						Module:        moduleNameOf(f),
					},
					Content: []byte(mf.Content),
				})
			}
		} else {
			// One template → 1 file (LAZY)
			result = append(result, &TemplateFile{
				BasicFile: BasicFile{
					RelPathStr:    relPath,
					TargetBaseDir: f.TargetBase(),
					FileMode:      f.Mode(), // Usually templates produce non-exec files but we can keep source mode
					Info:          f.SourceInfo(),
					FileType:      TypeTemplate,
					Module:        moduleNameOf(f),
				},
				SourceFile: f,
				Engine:     p.engine,
				Data:       templateData,
				Context:    ctx,
			})
		}
	}

	return result, nil
}
