package godotyaml

import (
	"fmt"

	"go.yaml.in/yaml/v3"
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
// The section is REPLACED, not merged: every key currently under external.<name>
// is discarded, including keys the caller did not supply and any comment written
// by hand inside the section. A comment attached to the external.<name> key
// itself survives, because that key node is reused. Use MergeExternalConfig when
// the section's other keys and comments have to be kept.
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
	if i := mappingIndex(ext, name); i >= 0 {
		ext.Content[i+1] = valNode // replace value, keep key node and its comments
		return nil
	}
	appendMappingKey(ext, name, valNode)
	return nil
}

// merges value into external.<name>, creating the section if needed, keeping the
// keys and comments the caller did not supply.
//
// Mappings are merged recursively at every depth: a key present in value takes
// the value given, and a key absent from value is left exactly as it was,
// together with any comment attached to it. Sequences and scalars are not merged
// but replaced wholesale, which is what lets a caller shorten or clear a list by
// supplying the new one.
//
// A merge never removes anything. A tool that drops a key from its own config
// struct and then merges will still find the old key in the file, because
// "absent from value" means "leave it alone", not "delete it". To remove
// something use RemoveExternalConfigKey for a single key, SetExternalConfigKey to
// replace one top-level key of the section wholesale, or SetExternalConfig to
// replace the whole section.
//
// Comments already in the file survive the merge. A comment carried on value (for
// callers that build their own *yaml.Node) replaces the comment on the matching
// key, but an empty comment never erases one that is already in the file.
//
// Only external.<name> is touched: sibling sections and the remainder of the
// file remain intact and untouched.
func (d *Document) MergeExternalConfig(name string, value any) error {
	if d.Root.Kind != yaml.MappingNode {
		return fmt.Errorf("godotyaml: cannot merge external config %q: document root is not a mapping", name)
	}

	valNode, err := toNode(value)
	if err != nil {
		return err
	}

	ext := d.ensureExternal()
	if i := mappingIndex(ext, name); i >= 0 {
		ext.Content[i+1] = mergeNodes(ext.Content[i+1], valNode)
		return nil
	}
	appendMappingKey(ext, name, valNode)
	return nil
}

// writes external.<name>.<key>, creating the section (and external) if needed.
//
// The value at key is replaced wholesale, so any mapping previously stored there
// is discarded along with the comments attached to that value. A comment attached
// to the key itself is kept, as is every other key of the section. Use this to
// replace one top-level key of a section outright where MergeExternalConfig would
// merge into it.
//
// If external.<name> exists but does not hold a mapping, it is replaced by one.
func (d *Document) SetExternalConfigKey(name, key string, value any) error {
	if d.Root.Kind != yaml.MappingNode {
		return fmt.Errorf("godotyaml: cannot set external config key %q.%q: document root is not a mapping", name, key)
	}

	valNode, err := toNode(value)
	if err != nil {
		return err
	}

	sec := d.ensureExternalSection(name)
	if i := mappingIndex(sec, key); i >= 0 {
		sec.Content[i+1] = valNode // replace value, keep key node and its comments
		return nil
	}
	appendMappingKey(sec, key, valNode)
	return nil
}

// removes external.<name>, reporting whether the section was present.
//
// The external mapping itself is never removed, even when the section removed was
// the last one. An empty external renders as "external: {}", which keeps any
// comment written above external; removing the mapping as well would silently
// discard that comment, which is the kind of loss this library exists to avoid.
// A later write repopulates the empty mapping in block style as usual.
func (d *Document) RemoveExternalConfig(name string) bool {
	ext := d.externalMapping()
	i := mappingIndex(ext, name)
	if i < 0 {
		return false
	}
	ext.Content = append(ext.Content[:i], ext.Content[i+2:]...)
	return true
}

// removes external.<name>.<key>, reporting whether the key was present.
//
// The comments attached to that key go with it. Neighbouring keys, the rest of
// the section, and the rest of the file are untouched. Removing the last key of a
// section leaves an empty section behind; use RemoveExternalConfig to remove the
// section itself.
func (d *Document) RemoveExternalConfigKey(name, key string) bool {
	sec := mappingValue(d.externalMapping(), name)
	i := mappingIndex(sec, key)
	if i < 0 {
		return false
	}
	sec.Content = append(sec.Content[:i], sec.Content[i+2:]...)
	return true
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
// non-mapping value) if necessary. The caller must have verified d.Root is a
// mapping.
func (d *Document) ensureExternal() *yaml.Node {
	if i := mappingIndex(d.Root, "external"); i >= 0 {
		if d.Root.Content[i+1].Kind != yaml.MappingNode {
			d.Root.Content[i+1] = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		}
		return d.Root.Content[i+1]
	}
	ext := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	appendMappingKey(d.Root, "external", ext)
	return ext
}

// ensureExternalSection returns the mapping node for external.<name>, creating it
// (or replacing a non-mapping value) if necessary. The caller must have verified
// d.Root is a mapping.
func (d *Document) ensureExternalSection(name string) *yaml.Node {
	ext := d.ensureExternal()
	if i := mappingIndex(ext, name); i >= 0 {
		if ext.Content[i+1].Kind != yaml.MappingNode {
			ext.Content[i+1] = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		}
		return ext.Content[i+1]
	}
	sec := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	appendMappingKey(ext, name, sec)
	return sec
}

// mergeNodes merges src into dst and returns the node that takes dst's place.
// Two mappings are merged recursively; any other combination is replaced by src,
// which inherits dst's comments wherever it carries none of its own.
func mergeNodes(dst, src *yaml.Node) *yaml.Node {
	if dst == nil {
		return src
	}
	if dst.Kind != yaml.MappingNode || src.Kind != yaml.MappingNode {
		inheritComments(src, dst)
		return src
	}

	for i := 0; i+1 < len(src.Content); i += 2 {
		sk, sv := src.Content[i], src.Content[i+1]
		if j := mappingIndex(dst, sk.Value); j >= 0 {
			overwriteComments(dst.Content[j], sk) // head comments live on the key node
			dst.Content[j+1] = mergeNodes(dst.Content[j+1], sv)
			continue
		}
		appendMapping(dst, sk, sv)
	}
	return dst
}

// mappingIndex returns the index of the key node for key in mapping m, or -1 if m
// is not a mapping or the key is absent. Its value node sits at index+1.
func mappingIndex(m *yaml.Node, key string) int {
	if m == nil || m.Kind != yaml.MappingNode {
		return -1
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return i
		}
	}
	return -1
}

// appendMappingKey appends a key/value pair to mapping m, creating the key node.
func appendMappingKey(m *yaml.Node, key string, val *yaml.Node) {
	appendMapping(m, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, val)
}

// appendMapping appends an existing key/value node pair to mapping m.
//
// An empty mapping is always emitted as "{}", so it comes back from a reparse in
// flow style, and yaml.v3 forces flow style onto every descendant of a flow node.
// Dropping the style as the first content is added keeps a document whose root,
// whose external, or whose section is currently empty from being rewritten as a
// single inline line.
func appendMapping(m *yaml.Node, key, val *yaml.Node) {
	if len(m.Content) == 0 {
		m.Style = 0
	}
	m.Content = append(m.Content, key, val)
}

// inheritComments gives n any comment it does not carry itself from prev, so a
// replacement value keeps the comments written against the value it replaces.
func inheritComments(n, prev *yaml.Node) {
	if n.HeadComment == "" {
		n.HeadComment = prev.HeadComment
	}
	if n.LineComment == "" {
		n.LineComment = prev.LineComment
	}
	if n.FootComment == "" {
		n.FootComment = prev.FootComment
	}
}

// overwriteComments applies src's non-empty comments to n, so a caller that
// builds its own nodes can update the comments it emits without an empty comment
// erasing what is already in the file.
func overwriteComments(n, src *yaml.Node) {
	if src.HeadComment != "" {
		n.HeadComment = src.HeadComment
	}
	if src.LineComment != "" {
		n.LineComment = src.LineComment
	}
	if src.FootComment != "" {
		n.FootComment = src.FootComment
	}
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
