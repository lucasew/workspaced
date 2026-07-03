package backup

import (
	"encoding/json"
	"fmt"
)

var actionDecoders = map[string]func(raw json.RawMessage) (BackupAction, error){}

func registerAction[T BackupAction](kind string) {
	actionDecoders[kind] = func(raw json.RawMessage) (BackupAction, error) {
		var a T
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, fmt.Errorf("decode %s action: %w", kind, err)
		}
		return a, nil
	}
}
