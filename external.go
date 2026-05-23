package godotyaml

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// returns the raw yaml.Node for external.<name>, reporting whether the
// section exists.
//
// The node is returned so the caller can decode it into its own types
// (node.Decode(&out)) without godotyaml imposing a structure on it. The node is
// the live tree node; treat it as read-only and use SetExternalConfig to write changes.
func (d *Document) GetRawExternalConfig(name string) (*yaml.Node, bool) {
	v := mappingValue(d.externalMapping(), name)
	if v == nil {
		return nil, false
	}
	return v, true
}

// decodes external.<name> into out, reporting whether the section
// exists. It is a thin convenience over GetRawExternalConfig followed by node.Decode.
func (d *Document) DecodeExternalConfig(name string, out any) (bool, error) {
	v, ok := d.GetRawExternalConfig(name)
	if !ok {
		return false, nil
	}
	if err := v.Decode(out); err != nil {
		return true, err
	}
	return true, nil
}

// returns the config names present under external, in document order.
func (d *Document) ExternalConfigNames() []string {
	ext := d.externalMapping()
	if ext == nil || ext.Kind != yaml.MappingNode {
		return nil
	}
	names := make([]string, 0, len(ext.Content)/2)
	for i := 0; i+1 < len(ext.Content); i += 2 {
		names = append(names, ext.Content[i].Value)
	}
	return names
}

// writes external.<name>, creating the external section if needed.
//
// value may be any value yaml.v3 can marshal, or an existing *yaml.Node for
// callers that manage their own subtree (e.g. to preserve their own comments).
//
// This function only touches the target section of `external` and sibling
// objects and the remainder of the file remains intact and untouched
func (d *Document) SetExternalConfig(name string, value any) error {
	if d.Root.Kind != yaml.MappingNode {
		return fmt.Errorf("godotyaml: cannot set external config %q: document root is not a mapping", name)
	}

	valNode, err := toNode(value)
	if err != nil {
		return err
	}

	ext := d.ensureExternal()
	for i := 0; i+1 < len(ext.Content); i += 2 {
		if ext.Content[i].Value == name {
			ext.Content[i+1] = valNode // replace value, keep key node and its comments
			return nil
		}
	}
	key := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: name}
	ext.Content = append(ext.Content, key, valNode)
	return nil
}

// externalMapping returns the external value node if it is a mapping, else nil.
func (d *Document) externalMapping() *yaml.Node {
	v := mappingValue(d.Root, "external")
	if v == nil || v.Kind != yaml.MappingNode {
		return nil
	}
	return v
}

// ensureExternal returns the external mapping node, creating it (or replacing a
// non-mapping value) if necessary. The caller must have verified d.root is a
// mapping.
func (d *Document) ensureExternal() *yaml.Node {
	for i := 0; i+1 < len(d.Root.Content); i += 2 {
		if d.Root.Content[i].Value == "external" {
			if d.Root.Content[i+1].Kind != yaml.MappingNode {
				d.Root.Content[i+1] = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			}
			return d.Root.Content[i+1]
		}
	}
	ext := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	key := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "external"}
	d.Root.Content = append(d.Root.Content, key, ext)
	return ext
}

// toNode converts an arbitrary value into a yaml node suitable as a mapping
// value. An existing *yaml.Node is used directly (unwrapping a document node),
// preserving any comments the caller attached.
func toNode(value any) (*yaml.Node, error) {
	if n, ok := value.(*yaml.Node); ok {
		if n.Kind == yaml.DocumentNode && len(n.Content) == 1 {
			return n.Content[0], nil
		}
		return n, nil
	}
	var n yaml.Node
	if err := n.Encode(value); err != nil {
		return nil, err
	}
	return &n, nil
}
