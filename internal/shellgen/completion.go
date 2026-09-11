package shellgen

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

var (
	// ErrRootCommandNotSet is returned when shell completion is requested before setting the root spec.
	ErrRootCommandNotSet = errors.New("root spec not set, call SetRootSpec first")
)

// GenerateCompletion generates bash completion from the x/cmd spec.
func GenerateCompletion() (string, error) {
	if rootSpec == nil {
		return "", ErrRootCommandNotSet
	}
	t := reflect.TypeOf(rootSpec)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	cmds, flags := collectComp(t)
	var b strings.Builder
	b.WriteString("# workspaced completion (x/cmd)\n")
	b.WriteString("_workspaced() {\n")
	b.WriteString("  local cur prev\n")
	b.WriteString("  COMPREPLY=()\n")
	b.WriteString("  cur=\"${COMP_WORDS[COMP_CWORD]}\"\n")
	b.WriteString("  prev=\"${COMP_WORDS[COMP_CWORD-1]}\"\n")
	fmt.Fprintf(&b, "  local cmds=%q\n", strings.Join(cmds, " "))
	fmt.Fprintf(&b, "  local flags=%q\n", strings.Join(flags, " "))
	b.WriteString("  if [[ \"$cur\" == -* ]]; then\n")
	b.WriteString("    COMPREPLY=( $(compgen -W \"$flags\" -- \"$cur\") )\n")
	b.WriteString("    return\n")
	b.WriteString("  fi\n")
	b.WriteString("  COMPREPLY=( $(compgen -W \"$cmds\" -- \"$cur\") )\n")
	b.WriteString("}\n")
	b.WriteString("complete -F _workspaced workspaced\n")
	return b.String(), nil
}

func collectComp(t reflect.Type) (cmds, flags []string) {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, nil
	}
	seenCmd := map[string]bool{}
	seenFlag := map[string]bool{}
	var walk func(reflect.Type)
	walk = func(t reflect.Type) {
		if t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		if t.Kind() != reflect.Struct {
			return
		}
		for i := range t.NumField() {
			sf := t.Field(i)
			ft := sf.Type
			_, flatten := sf.Tag.Lookup("flatten")
			if (sf.Anonymous || flatten) && (ft.Kind() == reflect.Struct || ft.Kind() == reflect.Pointer && ft.Elem().Kind() == reflect.Struct) {
				walk(ft)
				continue
			}
			if name := sf.Tag.Get("cmd"); name != "" {
				if !seenCmd[name] {
					seenCmd[name] = true
					cmds = append(cmds, name)
				}
				continue
			}
			if ft.Kind() == reflect.Pointer && ft.Elem().Kind() == reflect.Struct {
				n := strings.ToLower(sf.Name)
				if n != "" && !seenCmd[n] {
					seenCmd[n] = true
					cmds = append(cmds, n)
				}
				continue
			}
			if long := sf.Tag.Get("long"); long != "" && !seenFlag["--"+long] {
				seenFlag["--"+long] = true
				flags = append(flags, "--"+long)
			}
			if short := sf.Tag.Get("short"); short != "" && !seenFlag["-"+short] {
				seenFlag["-"+short] = true
				flags = append(flags, "-"+short)
			}
		}
	}
	walk(t)
	return cmds, flags
}
