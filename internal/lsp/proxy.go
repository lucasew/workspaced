package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"workspaced/internal/configcue"
	"workspaced/pkg/logging"
)

// Proxy is the editor-facing LSP router.
type Proxy struct {
	client  *Conn
	cfg     Config
	root    string
	rootURI string
	docs    *DocStore
	ctx     context.Context // root ctx for backend goroutines / logging

	mu       sync.Mutex
	backends map[string]*Backend // serverID -> backend
	// language -> serverIDs started for it (for sync fan-out)
	langServers map[string][]LanguageBinding
	initParams  json.RawMessage // client initialize params for backend init

	timeout time.Duration
}

// Run starts the proxy on the given client streams until exit/shutdown.
func Run(ctx context.Context, r io.Reader, w io.Writer) error {
	p := &Proxy{
		client:      NewConn(r, w),
		docs:        NewDocStore(),
		backends:    map[string]*Backend{},
		langServers: map[string][]LanguageBinding{},
		timeout:     defaultRequestTimeout,
		ctx:         ctx,
	}
	return p.loop(ctx)
}

func (p *Proxy) loop(ctx context.Context) error {
	logger := logging.GetLogger(ctx)
	for {
		msg, err := p.client.ReadMessage()
		if err != nil {
			if err == io.EOF {
				p.closeAll()
				return nil
			}
			return err
		}
		if err := p.handleClient(ctx, msg); err != nil {
			if err == io.EOF {
				p.closeAll()
				return nil
			}
			logger.Error("lsp handle client", "method", msg.Method, "error", err)
			if msg.IsRequest() {
				_ = p.client.WriteError(msg.ID, CodeInternalError, err.Error())
			}
		}
	}
}

func (p *Proxy) handleClient(ctx context.Context, msg *Message) error {
	switch {
	case msg.IsRequest():
		return p.handleRequest(ctx, msg)
	case msg.IsNotification():
		return p.handleNotification(ctx, msg)
	default:
		return nil
	}
}

func (p *Proxy) handleRequest(ctx context.Context, msg *Message) error {
	switch msg.Method {
	case "initialize":
		return p.onInitialize(ctx, msg)
	case "shutdown":
		p.closeAll()
		return p.client.WriteResult(msg.ID, nil)
	case "exit":
		p.closeAll()
		return io.EOF
	default:
		return p.forwardRequest(ctx, msg)
	}
}

func (p *Proxy) handleNotification(ctx context.Context, msg *Message) error {
	switch msg.Method {
	case "initialized":
		return nil
	case "exit":
		p.closeAll()
		return io.EOF
	case "textDocument/didOpen":
		return p.onDidOpen(ctx, msg)
	case "textDocument/didChange":
		return p.onDidChange(ctx, msg)
	case "textDocument/didClose":
		return p.onDidClose(ctx, msg)
	case "textDocument/didSave":
		return p.fanoutNotify(ctx, msg, true)
	case "$/cancelRequest":
		// Best-effort ignore for v1.
		return nil
	default:
		// Workspace or other notifications: fan-out to all live backends.
		return p.fanoutNotifyAll(ctx, msg)
	}
}

func (p *Proxy) onInitialize(ctx context.Context, msg *Message) error {
	logger := logging.GetLogger(ctx)

	var params struct {
		RootURI          string `json:"rootUri"`
		RootPath         string `json:"rootPath"`
		WorkspaceFolders []struct {
			URI  string `json:"uri"`
			Name string `json:"name"`
		} `json:"workspaceFolders"`
		Capabilities json.RawMessage `json:"capabilities"`
	}
	if len(msg.Params) > 0 {
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			return p.client.WriteError(msg.ID, CodeInvalidParams, err.Error())
		}
	}

	if len(params.WorkspaceFolders) > 1 {
		return p.client.WriteError(msg.ID, CodeInvalidParams, "workspaced lsp requires a single workspace folder")
	}

	rootURI := params.RootURI
	if rootURI == "" && len(params.WorkspaceFolders) == 1 {
		rootURI = params.WorkspaceFolders[0].URI
	}
	if rootURI == "" && params.RootPath != "" {
		rootURI = pathToURI(params.RootPath)
	}
	if rootURI == "" {
		return p.client.WriteError(msg.ID, CodeInvalidParams, "missing rootUri")
	}

	root, err := filepath.Abs(uriToPath(rootURI))
	if err != nil {
		return p.client.WriteError(msg.ID, CodeInvalidParams, err.Error())
	}

	// Load codebase config at client root (empty lsp block is fine).
	cueCfg, err := configcue.LoadForWorkspace(ctx, root)
	if err != nil {
		logger.Warn("lsp: load codebase config failed; continuing with empty lsp routes", "root", root, "error", err)
		p.cfg = Config{}
	} else {
		cfg, err := LoadConfig(cueCfg)
		if err != nil {
			return p.client.WriteError(msg.ID, CodeInternalError, err.Error())
		}
		p.cfg = cfg
	}
	p.timeout = p.cfg.Timeout()
	p.root = root
	p.rootURI = rootURI
	p.initParams = msg.Params

	logger.Info("lsp initialized", "root", root, "servers", len(p.cfg.Servers), "languages", len(p.cfg.Languages))
	return p.client.WriteResult(msg.ID, initializeResult(rootURI))
}

func (p *Proxy) onDidOpen(ctx context.Context, msg *Message) error {
	var params struct {
		TextDocument struct {
			URI        string `json:"uri"`
			LanguageID string `json:"languageId"`
			Version    int    `json:"version"`
			Text       string `json:"text"`
		} `json:"textDocument"`
	}
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return err
	}
	td := params.TextDocument
	lang := p.cfg.ResolveLanguage(td.URI, td.LanguageID)
	p.docs.Put(&Document{
		URI:        td.URI,
		LanguageID: td.LanguageID,
		Language:   lang,
		Version:    td.Version,
		Text:       td.Text,
	})
	if lang == "" {
		return nil
	}
	if err := p.ensureLanguage(ctx, lang); err != nil {
		logging.GetLogger(ctx).Warn("lsp ensure language", "language", lang, "error", err)
		return nil
	}
	// Async full sync fan-out.
	go p.syncNotify(ctx, lang, "textDocument/didOpen", msg.Params)
	return nil
}

func (p *Proxy) onDidChange(ctx context.Context, msg *Message) error {
	var params struct {
		TextDocument struct {
			URI     string `json:"uri"`
			Version int    `json:"version"`
		} `json:"textDocument"`
		ContentChanges []struct {
			Text string `json:"text"`
		} `json:"contentChanges"`
	}
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return err
	}
	doc, ok := p.docs.Get(params.TextDocument.URI)
	if !ok {
		return nil
	}
	// Full sync from client (we advertise change=1 Full). Last change text is whole buffer.
	if n := len(params.ContentChanges); n > 0 {
		doc.Text = params.ContentChanges[n-1].Text
	}
	doc.Version = params.TextDocument.Version
	p.docs.Put(doc)
	if doc.Language == "" {
		return nil
	}
	// Rebuild full-sync notify for backends (always full text).
	fullParams := map[string]any{
		"textDocument": map[string]any{
			"uri":     doc.URI,
			"version": doc.Version,
		},
		"contentChanges": []map[string]any{
			{"text": doc.Text},
		},
	}
	raw, _ := json.Marshal(fullParams)
	go p.syncNotify(ctx, doc.Language, "textDocument/didChange", raw)
	return nil
}

func (p *Proxy) onDidClose(ctx context.Context, msg *Message) error {
	var params struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
	}
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return err
	}
	doc, ok := p.docs.Get(params.TextDocument.URI)
	lang := ""
	if ok {
		lang = doc.Language
	}
	p.docs.Delete(params.TextDocument.URI)
	if lang != "" {
		go p.syncNotify(ctx, lang, "textDocument/didClose", msg.Params)
	}
	return nil
}

func (p *Proxy) syncNotify(ctx context.Context, lang, method string, params json.RawMessage) {
	bindings := p.bindingsLive(lang)
	var wg sync.WaitGroup
	for _, b := range bindings {
		backend := p.getBackend(b.ServerID)
		if backend == nil {
			continue
		}
		wg.Add(1)
		go func(backend *Backend) {
			defer wg.Done()
			var raw any
			_ = json.Unmarshal(params, &raw)
			if err := backend.Notify(method, raw); err != nil {
				logging.GetLogger(ctx).Debug("lsp sync notify", "server", backend.ServerID, "method", method, "error", err)
			}
		}(backend)
	}
	wg.Wait()
}

func (p *Proxy) fanoutNotify(ctx context.Context, msg *Message, requireDoc bool) error {
	uri := extractURI(msg.Params)
	lang := ""
	if uri != "" {
		if doc, ok := p.docs.Get(uri); ok {
			lang = doc.Language
		} else if requireDoc {
			lang = p.cfg.ResolveLanguage(uri, "")
		}
	}
	if lang == "" {
		return p.fanoutNotifyAll(ctx, msg)
	}
	go p.syncNotify(ctx, lang, msg.Method, msg.Params)
	return nil
}

func (p *Proxy) fanoutNotifyAll(ctx context.Context, msg *Message) error {
	p.mu.Lock()
	backends := make([]*Backend, 0, len(p.backends))
	for _, b := range p.backends {
		backends = append(backends, b)
	}
	p.mu.Unlock()
	var raw any
	_ = json.Unmarshal(msg.Params, &raw)
	for _, b := range backends {
		backend := b
		go func() {
			if err := backend.Notify(msg.Method, raw); err != nil {
				logging.GetLogger(ctx).Debug("lsp notify all", "server", backend.ServerID, "error", err)
			}
		}()
	}
	return nil
}

func (p *Proxy) forwardRequest(ctx context.Context, msg *Message) error {
	cap := CapabilityForMethod(msg.Method)
	uri := extractURI(msg.Params)
	lang := ""
	if uri != "" {
		if doc, ok := p.docs.Get(uri); ok {
			lang = doc.Language
		} else {
			lang = p.cfg.ResolveLanguage(uri, "")
		}
	}

	// Workspace-scoped methods without a document: all languages' servers that claim the cap.
	var targets []*Backend
	if lang != "" {
		if err := p.ensureLanguage(ctx, lang); err != nil {
			logging.GetLogger(ctx).Warn("lsp ensure language", "language", lang, "error", err)
		}
		for _, b := range p.bindingsLive(lang) {
			if !b.HasCapability(cap) {
				continue
			}
			if backend := p.getBackend(b.ServerID); backend != nil {
				targets = append(targets, backend)
			}
		}
	} else {
		// Try every live backend whose any binding has the cap — for workspace/* etc.
		p.mu.Lock()
		for _, backend := range p.backends {
			targets = append(targets, backend)
		}
		p.mu.Unlock()
		// Filter by capability using any language binding that uses this server.
		filtered := targets[:0]
		for _, backend := range targets {
			if p.serverHasCapability(backend.ServerID, cap) {
				filtered = append(filtered, backend)
			}
		}
		targets = filtered
	}

	if len(targets) == 0 {
		return p.client.WriteError(msg.ID, CodeMethodNotFound, fmt.Sprintf("unsupported: %s", msg.Method))
	}

	var rawParams any
	if len(msg.Params) > 0 {
		_ = json.Unmarshal(msg.Params, &rawParams)
	}

	type one struct {
		msg *Message
		err error
	}
	results := make([]one, len(targets))
	var wg sync.WaitGroup
	for i, backend := range targets {
		wg.Add(1)
		go func(i int, backend *Backend) {
			defer wg.Done()
			reqCtx, cancel := context.WithTimeout(ctx, p.timeout)
			defer cancel()
			resp, err := backend.Request(reqCtx, msg.Method, rawParams)
			results[i] = one{msg: resp, err: err}
		}(i, backend)
	}
	wg.Wait()

	var merged []json.RawMessage
	var lastErr error
	for _, r := range results {
		if r.err != nil {
			lastErr = r.err
			continue // soft timeout / failure: exclude from this merge
		}
		if r.msg == nil {
			continue
		}
		if r.msg.Error != nil {
			lastErr = fmt.Errorf("%s", r.msg.Error.Message)
			continue
		}
		if len(r.msg.Result) > 0 {
			merged = append(merged, r.msg.Result)
		}
	}
	if len(merged) == 0 {
		if lastErr != nil {
			return p.client.WriteError(msg.ID, CodeRequestFailed, lastErr.Error())
		}
		return p.client.WriteError(msg.ID, CodeMethodNotFound, fmt.Sprintf("unsupported: %s", msg.Method))
	}

	out := mergeResults(merged)
	return p.client.WriteMessage(&Message{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result:  out,
	})
}

func (p *Proxy) serverHasCapability(serverID, cap string) bool {
	for _, bindings := range p.langServers {
		for _, b := range bindings {
			if b.ServerID == serverID && b.HasCapability(cap) {
				return true
			}
		}
	}
	// If we don't track bindings yet, allow.
	return len(p.langServers) == 0
}

func (p *Proxy) ensureLanguage(ctx context.Context, lang string) error {
	p.mu.Lock()
	if _, ok := p.langServers[lang]; ok {
		p.mu.Unlock()
		return nil
	}
	// Mark in-progress so concurrent callers wait on the same start.
	p.langServers[lang] = nil
	p.mu.Unlock()

	bindings := p.cfg.BindingsFor(lang)
	if len(bindings) == 0 {
		return nil
	}

	for _, b := range bindings {
		p.mu.Lock()
		existing := p.backends[b.ServerID]
		p.mu.Unlock()
		if existing != nil {
			continue
		}
		srv, ok := p.cfg.Servers[b.ServerID]
		if !ok {
			return fmt.Errorf("language %q references unknown server %q", lang, b.ServerID)
		}
		backend, err := StartBackend(ctx, p.root, b.ServerID, srv, p.onBackendMessage)
		if err != nil {
			return err
		}
		if err := p.initializeBackend(ctx, backend); err != nil {
			backend.Close()
			return fmt.Errorf("initialize %q: %w", b.ServerID, err)
		}
		// Replay open docs for this language.
		for _, doc := range p.docs.ByLanguage(lang) {
			_ = backend.Notify("textDocument/didOpen", map[string]any{
				"textDocument": map[string]any{
					"uri":        doc.URI,
					"languageId": doc.LanguageID,
					"version":    doc.Version,
					"text":       doc.Text,
				},
			})
		}
		p.mu.Lock()
		p.backends[b.ServerID] = backend
		p.mu.Unlock()
	}

	p.mu.Lock()
	p.langServers[lang] = bindings
	p.mu.Unlock()
	return nil
}

func (p *Proxy) initializeBackend(ctx context.Context, backend *Backend) error {
	// Build initialize params from client, forcing our root.
	var base map[string]any
	if len(p.initParams) > 0 {
		_ = json.Unmarshal(p.initParams, &base)
	}
	if base == nil {
		base = map[string]any{}
	}
	base["rootUri"] = p.rootURI
	base["rootPath"] = p.root
	base["workspaceFolders"] = []map[string]any{
		{"uri": p.rootURI, "name": filepath.Base(p.root)},
	}
	// processId null so backends don't watch our pid incorrectly? Keep as is from client.

	reqCtx, cancel := context.WithTimeout(ctx, p.timeout*2)
	defer cancel()
	resp, err := backend.Request(reqCtx, "initialize", base)
	if err != nil {
		return err
	}
	if resp.Error != nil {
		return fmt.Errorf("initialize error: %s", resp.Error.Message)
	}
	return backend.Notify("initialized", map[string]any{})
}

func (p *Proxy) onBackendMessage(serverID string, msg *Message) {
	// Server->client notifications and requests: forward as-is (dogfood).
	// For requests, we need to reply — v1 only forwards notifications to avoid ID chaos.
	if msg.IsRequest() {
		// Respond with method not found so backends don't hang; log.
		_ = msg
		go func() {
			// Soft: ignore server-initiated requests for v1 (configuration etc. may break some servers).
			// Try to answer workspace/configuration with empty items if we can.
			if msg.Method == "workspace/configuration" {
				var params struct {
					Items []json.RawMessage `json:"items"`
				}
				_ = json.Unmarshal(msg.Params, &params)
				result := make([]any, len(params.Items))
				raw, _ := json.Marshal(result)
				p.mu.Lock()
				b := p.backends[serverID]
				p.mu.Unlock()
				if b != nil {
					_ = b.conn.WriteMessage(&Message{
						JSONRPC: "2.0",
						ID:      msg.ID,
						Result:  raw,
					})
				}
				return
			}
			if msg.Method == "window/workDoneProgress/create" {
				p.mu.Lock()
				b := p.backends[serverID]
				p.mu.Unlock()
				if b != nil {
					_ = b.conn.WriteMessage(&Message{
						JSONRPC: "2.0",
						ID:      msg.ID,
						Result:  json.RawMessage("null"),
					})
				}
				return
			}
			// client/registerCapability
			if msg.Method == "client/registerCapability" || msg.Method == "client/unregisterCapability" {
				p.mu.Lock()
				b := p.backends[serverID]
				p.mu.Unlock()
				if b != nil {
					_ = b.conn.WriteMessage(&Message{
						JSONRPC: "2.0",
						ID:      msg.ID,
						Result:  json.RawMessage("null"),
					})
				}
				return
			}
			logging.GetLogger(p.ctx).Debug("lsp ignoring backend request", "server", serverID, "method", msg.Method)
		}()
		return
	}
	// Notifications (diagnostics, logMessage, …)
	_ = p.client.WriteMessage(msg)
}

func (p *Proxy) bindingsLive(lang string) []LanguageBinding {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.langServers[lang]
}

func (p *Proxy) getBackend(serverID string) *Backend {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.backends[serverID]
}

func (p *Proxy) closeAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for id, b := range p.backends {
		// best-effort shutdown
		_, _ = b.Request(context.Background(), "shutdown", nil)
		_ = b.Notify("exit", nil)
		b.Close()
		delete(p.backends, id)
	}
	p.langServers = map[string][]LanguageBinding{}
}

func extractURI(params json.RawMessage) string {
	if len(params) == 0 {
		return ""
	}
	var probe struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(params, &probe); err != nil {
		return ""
	}
	if probe.TextDocument.URI != "" {
		return probe.TextDocument.URI
	}
	return probe.URI
}

// RootFromURI is exported for tests.
func RootFromURI(u string) (string, error) {
	if strings.HasPrefix(u, "file://") {
		parsed, err := url.Parse(u)
		if err != nil {
			return "", err
		}
		return filepath.Abs(parsed.Path)
	}
	return filepath.Abs(u)
}
