package godotyaml

import (
	"bytes"
	"errors"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// Document is a parsed go.yaml file.
//
// The underlying yaml.v3 node tree is the source of truth: typed accessors read
// from it on demand, and mutations edit it in place. Keeping the node tree
// canonical (rather than unmarshaling into a struct) is what lets the library
// preserve comments, key ordering, and unknown keys across a load/save cycle.
type Document struct {
	doc  *yaml.Node // the document node returned by the decoder
	root *yaml.Node // the root mapping node (doc.Content[0])
}

// Parse reads a go.yaml document from r.
//
// An empty input yields an empty document with a writable root mapping rather
// than an error, so callers can build a file from scratch.
func Parse(r io.Reader) (*Document, error) {
	var node yaml.Node
	if err := yaml.NewDecoder(r).Decode(&node); err != nil {
		if errors.Is(err, io.EOF) {
			return newEmpty(), nil
		}
		return nil, err
	}

	d := &Document{doc: &node}
	if len(node.Content) > 0 {
		d.root = node.Content[0]
	}
	if d.root == nil {
		root := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		d.doc.Content = []*yaml.Node{root}
		d.root = root
	}
	return d, nil
}

// Load reads and parses the go.yaml file at path.
func Load(path string) (*Document, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Parse(f)
}

// Write serializes the document to w, preserving comments and key ordering.
//
// Indentation is normalized to two spaces; yaml.v3 does not record the source
// file's original indent width, so that one stylistic detail is not preserved.
// Structure and ordering are.
func (d *Document) Write(w io.Writer) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	if err := enc.Encode(d.doc); err != nil {
		_ = enc.Close()
		return err
	}
	return enc.Close()
}

// Save writes the document back to the file at path.
//
// The file is rendered fully in memory first so a serialization error cannot
// leave a truncated file on disk.
func (d *Document) Save(path string) error {
	var buf bytes.Buffer
	if err := d.Write(&buf); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func newEmpty() *Document {
	root := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	doc := &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{root}}
	return &Document{doc: doc, root: root}
}

// mappingValue returns the value node for key in mapping m, or nil if m is not
// a mapping or the key is absent.
func mappingValue(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}
