package github

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type sourceMeta struct {
	URL  string `json:"url"`
	Hash string `json:"hash"`
}

const metaFilename = ".workspaced-source-meta.json"

func (s Source) WriteMeta(dir string, meta sourceMeta) error {
	b, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, metaFilename), b, 0644)
}

func (s Source) ReadMeta(dir string) (sourceMeta, error) {
	b, err := os.ReadFile(filepath.Join(dir, metaFilename))
	if err != nil {
		return sourceMeta{}, err
	}
	var meta sourceMeta
	if err := json.Unmarshal(b, &meta); err != nil {
		return sourceMeta{}, err
	}
	return meta, nil
}
