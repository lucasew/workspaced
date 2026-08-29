package lsp

import "sync"

// Document is a cached text document.
type Document struct {
	URI        string
	LanguageID string
	Language   string // our resolved language
	Version    int
	Text       string
}

// DocStore holds open documents.
type DocStore struct {
	mu   sync.RWMutex
	docs map[string]*Document
}

// NewDocStore creates an empty store.
func NewDocStore() *DocStore {
	return &DocStore{docs: map[string]*Document{}}
}

// Put inserts or replaces a document.
func (s *DocStore) Put(doc *Document) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *doc
	s.docs[doc.URI] = &cp
}

// Get returns a copy of the document if present.
func (s *DocStore) Get(uri string) (*Document, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.docs[uri]
	if !ok {
		return nil, false
	}
	cp := *d
	return &cp, true
}

// Delete removes a document.
func (s *DocStore) Delete(uri string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.docs, uri)
}

// All returns copies of every open document.
func (s *DocStore) All() []*Document {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Document, 0, len(s.docs))
	for _, d := range s.docs {
		cp := *d
		out = append(out, &cp)
	}
	return out
}

// ByLanguage returns documents for a language.
func (s *DocStore) ByLanguage(lang string) []*Document {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Document
	for _, d := range s.docs {
		if d.Language == lang {
			cp := *d
			out = append(out, &cp)
		}
	}
	return out
}
