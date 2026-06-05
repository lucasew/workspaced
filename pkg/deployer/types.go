package deployer

import (
	"path/filepath"
	"sort"
	"workspaced/pkg/source"
)

// ActionType representa tipo de ação no deployment
type ActionType int

const (
	ActionCreate ActionType = iota
	ActionUpdate
	ActionDelete
	ActionNoop
)

func (a ActionType) String() string {
	switch a {
	case ActionCreate:
		return "+"
	case ActionUpdate:
		return "*"
	case ActionDelete:
		return "-"
	case ActionNoop:
		return " "
	}
	return "?"
}

// DesiredState alias para source.DesiredState
type DesiredState = source.DesiredState

// Helper para pegar o target path de um DesiredState
func GetTarget(d DesiredState) string {
	return filepath.Join(d.File.TargetBase(), d.File.RelPath())
}

// ManagedInfo contém informações sobre arquivo gerenciado
type ManagedInfo struct {
	SourceInfo string `json:"source_info"`
}

// State representa estado atual do sistema
type State struct {
	Files map[string]ManagedInfo `json:"files"` // Key: Target (Absolute path)
}

// Action representa ação a ser executada
type Action struct {
	Type    ActionType
	Target  string
	Desired DesiredState
	Current ManagedInfo
}

func SortActions(actions []Action) []Action {
	ordered := append([]Action(nil), actions...)
	sort.SliceStable(ordered, func(i, j int) bool {
		rank := func(t ActionType) int {
			switch t {
			case ActionDelete:
				return 0
			case ActionUpdate:
				return 1
			case ActionCreate:
				return 2
			case ActionNoop:
				return 3
			default:
				return 4
			}
		}
		ri := rank(ordered[i].Type)
		rj := rank(ordered[j].Type)
		if ri != rj {
			return ri < rj
		}
		return ordered[i].Target < ordered[j].Target
	})
	return ordered
}

// Provider alias para source.Provider
type Provider = source.Provider
