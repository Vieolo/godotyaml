package godotyaml

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"go.yaml.in/yaml/v3"
)

// Document is a parsed go.yaml file.
//
// The underlying yaml.v3 node tree is the source of truth: typed accessors read
// from it on demand, and mutations edit it in place. Keeping the node tree
// canonical (rather than unmarshaling into a struct) is what lets the library
// preserve comments, key ordering, and unknown keys across a load/save cycle.
type Document struct {
	Doc  *yaml.Node // the document node returned by the decoder
	Root *yaml.Node // the root mapping node (doc.Content[0])
}

// reads a go.yaml document from the given io.Reader
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

	d := &Document{Doc: &node}
	if len(node.Content) > 0 {
		d.Root = node.Content[0]
	}
	if d.Root == nil {
		root := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		d.Doc.Content = []*yaml.Node{root}
		d.Root = root
	}
	return d, nil
}

// reads and parses the go.yaml file at given path
//
// When no file exists at path the returned error satisfies os.IsNotExist (and
// errors.Is(err, fs.ErrNotExist)), so a caller can tell "this project has no
// go.yaml" apart from "this go.yaml is broken" without stat-ing the path first.
func Load(path string) (*Document, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Parse(f)
}

// serializes the document to the given io.Writer, preserving comments and key ordering
//
// Indentation is normalized to two spaces; yaml.v3 does not record the source
// file's original indent width, so that one stylistic detail is not preserved.
// Structure and ordering are otherwise reproduced as they were read: key order,
// comments, unknown root keys, and every external section the caller did not
// touch are written back unchanged.
//
// Blank lines are NOT preserved. yaml.v3 has no representation for a blank line,
// so the empty lines a human used to separate keys and sections are all dropped
// the first time a tool writes the file. This is usually the most visible part of
// the diff a tool produces, and it is a limitation of the underlying YAML library
// rather than a choice godotyaml makes.
func (d *Document) Write(w io.Writer) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	if err := enc.Encode(d.Doc); err != nil {
		_ = enc.Close()
		return err
	}
	return enc.Close()
}

// writes the document back to the file at path.
//
// The write is atomic. The document is rendered fully in memory, written to a
// temporary file beside path, flushed, and then renamed over path, so a
// serialization error, a crash, or a full disk cannot leave a truncated go.yaml
// behind: path always holds either the previous content or the complete new
// content. A go.yaml usually carries several tools' configuration as well as the
// project metadata, so a half-written one is expensive to lose.
//
// An existing file keeps its permission bits exactly; because a rename takes the
// mode of the temporary file, Save copies the mode across itself. A file that
// Save creates is created 0644 as modified by the process umask. When path is a
// symlink the link is resolved and its target replaced, leaving the link itself
// in place.
func (d *Document) Save(path string) error {
	var buf bytes.Buffer
	if err := d.Write(&buf); err != nil {
		return err
	}

	// Rename replaces a symlink itself rather than writing through it, so
	// resolve first to keep the link intact.
	if fi, err := os.Lstat(path); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		if resolved, err := filepath.EvalSymlinks(path); err == nil {
			path = resolved
		}
	}

	// A new file is created 0644 (umask applies, as it would for os.WriteFile);
	// an existing file's own mode is reapplied after the temporary file is
	// created, so a 0600 go.yaml is not widened by the rename.
	perm, exists := os.FileMode(0), false
	if fi, err := os.Stat(path); err == nil {
		perm, exists = fi.Mode().Perm(), true
	}

	tmp := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".tmp"+strconv.Itoa(os.Getpid()))
	_ = os.Remove(tmp) // discard a leftover from an earlier crash so O_EXCL holds
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if exists {
		if err := f.Chmod(perm); err != nil {
			return errors.Join(err, f.Close(), os.Remove(tmp))
		}
	}
	if _, err := f.Write(buf.Bytes()); err != nil {
		return errors.Join(err, f.Close(), os.Remove(tmp))
	}
	if err := f.Sync(); err != nil {
		return errors.Join(err, f.Close(), os.Remove(tmp))
	}
	if err := f.Close(); err != nil {
		return errors.Join(err, os.Remove(tmp))
	}
	if err := os.Rename(tmp, path); err != nil {
		return errors.Join(err, os.Remove(tmp))
	}
	return nil
}

// writes the document to path only if no file is there yet.
//
// This is the write an `init` command wants. A go.yaml holds every tool's
// configuration as well as the project metadata, so overwriting one that already
// exists destroys other tools' data; SaveNew refuses instead of clobbering. If
// path exists the returned error satisfies os.IsExist (and errors.Is(err,
// fs.ErrExist)), which the caller can report as "this project already has a
// go.yaml".
//
// The check is not a stat followed by a write: the file is created exclusively,
// so two `init` runs racing each other cannot both decide the path was free. The
// content is then written with the same atomic replace Save uses.
func (d *Document) SaveNew(path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return errors.Join(err, os.Remove(path))
	}
	// path now exists and is ours, and is empty: a failure below leaves nothing
	// worth keeping, so it is removed rather than left as a stub that a later
	// init would refuse to overwrite.
	if err := d.Save(path); err != nil {
		return errors.Join(err, os.Remove(path))
	}
	return nil
}

func newEmpty() *Document {
	root := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	doc := &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{root}}
	return &Document{Doc: doc, Root: root}
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
