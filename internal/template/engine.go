package template

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"text/template"
)

var (
	ErrNotMultiFile = errors.New("template is not a multi-file template")
)

// Engine is the workspaced template rendering engine.
type Engine struct {
	funcMap template.FuncMap
}

// Option is a configuration function for Engine.
type Option func(*Engine)

// NewEngine creates a new template engine.
func NewEngine(ctx context.Context, opts ...Option) *Engine {
	e := &Engine{
		funcMap: makeFuncMap(ctx),
	}

	for _, opt := range opts {
		opt(e)
	}

	return e
}

// WithCustomFunc adds a custom function to the FuncMap.
func WithCustomFunc(name string, fn any) Option {
	return func(e *Engine) {
		e.funcMap[name] = fn
	}
}

// WithFuncMap replaces the entire FuncMap.
func WithFuncMap(funcMap template.FuncMap) Option {
	return func(e *Engine) {
		e.funcMap = funcMap
	}
}

// Render renders a template string with the provided data.
func (e *Engine) Render(ctx context.Context, tmpl string, data any) ([]byte, error) {
	t, err := template.New("template").Funcs(e.funcMap).Parse(tmpl)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		if errors.Is(err, ErrFileSkipped) {
			return nil, ErrFileSkipped
		}
		return nil, fmt.Errorf("execute template: %w", err)
	}

	return buf.Bytes(), nil
}

// RenderFile renders a template file from disk.
func (e *Engine) RenderFile(ctx context.Context, path string, data any) ([]byte, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read template file: %w", err)
	}

	return e.Render(ctx, string(content), data)
}

// RenderMultiFile renders a template and returns multiple output files.
func (e *Engine) RenderMultiFile(ctx context.Context, tmpl string, data any) ([]MultiFile, error) {
	rendered, err := e.Render(ctx, tmpl, data)
	if err != nil {
		return nil, err
	}

	files, isMulti := ParseMultiFile(rendered)
	if !isMulti {
		return nil, ErrNotMultiFile
	}

	return files, nil
}
