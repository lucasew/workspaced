package filespine

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"unicode"
	"unicode/utf8"

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
	case TypeJSON, TypeTOML, TypeYAML, TypeINI, TypeXML:
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
	case TypeXML:
		out, err = encodeXML(data)
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

type xmlEnc struct {
	buf *bytes.Buffer
}

func encodeXML(data map[string]any) ([]byte, error) {
	keys := sortedKeys(data)
	if len(keys) != 1 {
		return nil, fmt.Errorf("%w: got %d top-level keys", ErrXMLRoot, len(keys))
	}
	root := keys[0]
	if _, ok := data[root].([]any); ok {
		return nil, fmt.Errorf("%w: root %q is a list", ErrXMLRoot, root)
	}
	var buf bytes.Buffer
	if _, err := buf.WriteString(xml.Header); err != nil {
		return nil, err
	}
	enc := xmlEnc{buf: &buf}
	if err := enc.node(root, data[root], 0); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (e xmlEnc) node(name string, v any, indent int) error {
	if err := checkXMLName(name); err != nil {
		return err
	}
	switch x := v.(type) {
	case []any:
		for i, el := range x {
			if _, ok := el.([]any); ok {
				return fmt.Errorf("%s: %w: nested list", name, ErrUnsupportedValue)
			}
			if i > 0 {
				if err := e.buf.WriteByte('\n'); err != nil {
					return err
				}
			}
			if err := e.node(name, el, indent); err != nil {
				return err
			}
		}
		return nil
	case map[string]any:
		if err := e.indent(indent); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(e.buf, "<%s>", name); err != nil {
			return err
		}
		kids := sortedKeys(x)
		if len(kids) == 0 {
			_, err := fmt.Fprintf(e.buf, "</%s>", name)
			return err
		}
		if err := e.buf.WriteByte('\n'); err != nil {
			return err
		}
		for i, k := range kids {
			if i > 0 {
				if err := e.buf.WriteByte('\n'); err != nil {
					return err
				}
			}
			if err := e.node(k, x[k], indent+1); err != nil {
				return err
			}
		}
		if err := e.buf.WriteByte('\n'); err != nil {
			return err
		}
		if err := e.indent(indent); err != nil {
			return err
		}
		_, err := fmt.Fprintf(e.buf, "</%s>", name)
		return err
	default:
		s, err := xmlScalar(v)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if err := e.indent(indent); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(e.buf, "<%s>", name); err != nil {
			return err
		}
		if err := xml.EscapeText(e.buf, []byte(s)); err != nil {
			return err
		}
		_, err = fmt.Fprintf(e.buf, "</%s>", name)
		return err
	}
}

func (e xmlEnc) indent(n int) error {
	for range n {
		if _, err := e.buf.WriteString("  "); err != nil {
			return err
		}
	}
	return nil
}

func xmlScalar(v any) (string, error) {
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
		return "", fmt.Errorf("xml: %w: %T", ErrUnsupportedValue, v)
	}
}

func checkXMLName(name string) error {
	if name == "" || !validXMLName(name) {
		return fmt.Errorf("%w: %q", ErrXMLName, name)
	}
	return nil
}

func validXMLName(name string) bool {
	r, size := utf8.DecodeRuneInString(name)
	if size == 0 || r == utf8.RuneError || !isXMLNameStart(r) {
		return false
	}
	for _, r := range name[size:] {
		if !isXMLNameChar(r) {
			return false
		}
	}
	return true
}

func isXMLNameStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isXMLNameChar(r rune) bool {
	return r == '_' || r == '-' || r == '.' || unicode.IsLetter(r) || unicode.IsDigit(r)
}
