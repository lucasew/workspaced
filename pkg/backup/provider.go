package backup

import (
	"encoding/json"
	"fmt"
)

var actionProviders = map[string]func(raw json.RawMessage) (BackupAction, error){}

func registerActionProvider[T BackupAction](name string) {
	actionProviders[name] = func(raw json.RawMessage) (BackupAction, error) {
		var a T
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, fmt.Errorf("decode %s action: %w", name, err)
		}
		return a, nil
	}
}
