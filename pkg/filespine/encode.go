package filespine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	toml "github.com/pelletier/go-toml/v2"
	"go.yaml.in/yaml/v3"
)

// Encode writes the combined dest bytes for one file.
func Encode(f File) ([]byte, error) {
	if err := f.checkArity(); err != nil {
		return nil, fmt.Errorf("file %q: %w", f.Path, err)
	}
	var buf bytes.Buffer
	switch f.Type {
	case TypeLines:
		keys := f.keys()
		for i, key := range keys {
			if i > 0 {
				if err := buf.WriteByte('\n'); err != nil {
					return nil, err
				}
			}
			if err := f.Values[key].writeTo(&buf); err != nil {
				return nil, fmt.Errorf("file %q values.%s: %w", f.Path, key, err)
			}
		}
	case TypeText, TypeRef:
		if err := writeOnlySlot(&buf, f); err != nil {
			return nil, fmt.Errorf("file %q: %w", f.Path, err)
		}
	case TypeJSON, TypeTOML, TypeYAML, TypeINI:
		data, err := encodeStructured(f.Type, f.Data)
		if err != nil {
			return nil, fmt.Errorf("file %q: %w", f.Path, err)
		}
		return data, nil
	default:
		return nil, fmt.Errorf("file %q: %w: %s", f.Path, ErrUnknownFileType, f.Type)
	}
	return buf.Bytes(), nil
}

func writeOnlySlot(w io.Writer, f File) error {
	for _, key := range f.keys() {
		return f.Values[key].writeTo(w)
	}
	return fmt.Errorf("%w: got 0", ErrTextKeyCount)
}

func encodeStructured(typ string, data map[string]any) ([]byte, error) {
	if data == nil {
		data = map[string]any{}
	}
	var (
		out []byte
		err error
	)
	switch typ {
	case TypeJSON:
		out, err = json.MarshalIndent(data, "", "  ")
		if err != nil {
			return nil, err
		}
		out = append(out, '\n')
	case TypeTOML:
		out, err = toml.Marshal(data)
	case TypeYAML:
		out, err = encodeYAML(data)
	case TypeINI:
		out, err = encodeINI(data)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownFileType, typ)
	}
	if err != nil {
		return nil, err
	}
	return ensureNL(out), nil
}

func ensureNL(b []byte) []byte {
	if len(b) == 0 || b[len(b)-1] == '\n' {
		return b
	}
	return append(b, '\n')
}

func encodeYAML(data map[string]any) ([]byte, error) {
	node, err := yamlNode(data)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(node); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func yamlNode(v any) (*yaml.Node, error) {
	switch x := v.(type) {
	case map[string]any:
		n := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		for _, k := range sortedKeys(x) {
			n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k})
			child, err := yamlNode(x[k])
			if err != nil {
				return nil, err
			}
			n.Content = append(n.Content, child)
		}
		return n, nil
	case []any:
		n := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for _, el := range x {
			child, err := yamlNode(el)
			if err != nil {
				return nil, err
			}
			n.Content = append(n.Content, child)
		}
		return n, nil
	case string:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: x}, nil
	case bool:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(x)}, nil
	case int64:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.FormatInt(x, 10)}, nil
	case int:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(x)}, nil
	case float64:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!float", Value: strconv.FormatFloat(x, 'g', -1, 64)}, nil
	case nil:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}, nil
	default:
		return nil, fmt.Errorf("yaml: %w: %T", ErrUnsupportedValue, v)
	}
}

func encodeINI(data map[string]any) ([]byte, error) {
	var buf bytes.Buffer
	keys := sortedKeys(data)
	first := true
	for _, k := range keys {
		if _, ok := data[k].(map[string]any); ok {
			continue
		}
		line, err := iniLine(k, data[k])
		if err != nil {
			return nil, err
		}
		if !first {
			if err := buf.WriteByte('\n'); err != nil {
				return nil, err
			}
		}
		first = false
		if _, err := buf.WriteString(line); err != nil {
			return nil, err
		}
	}
	for _, k := range keys {
		sec, ok := data[k].(map[string]any)
		if !ok {
			continue
		}
		if !first {
			if err := buf.WriteByte('\n'); err != nil {
				return nil, err
			}
		}
		first = false
		if _, err := fmt.Fprintf(&buf, "[%s]", k); err != nil {
			return nil, err
		}
		sks := sortedKeys(sec)
		for _, sk := range sks {
			if _, nested := sec[sk].(map[string]any); nested {
				return nil, fmt.Errorf("%w: [%s].%s", ErrINISection, k, sk)
			}
			line, err := iniLine(sk, sec[sk])
			if err != nil {
				return nil, fmt.Errorf("[%s]: %w", k, err)
			}
			if _, err := buf.WriteString("\n" + line); err != nil {
				return nil, err
			}
		}
	}
	return buf.Bytes(), nil
}

func iniLine(key string, v any) (string, error) {
	s, err := iniScalar(v)
	if err != nil {
		return "", fmt.Errorf("%s: %w", key, err)
	}
	return key + " = " + s, nil
}

func iniScalar(v any) (string, error) {
	switch x := v.(type) {
	case string:
		return x, nil
	case bool:
		return strconv.FormatBool(x), nil
	case int64:
		return strconv.FormatInt(x, 10), nil
	case int:
		return strconv.Itoa(x), nil
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64), nil
	case nil:
		return "", nil
	default:
		return "", fmt.Errorf("ini: %w: %T", ErrUnsupportedValue, v)
	}
}
