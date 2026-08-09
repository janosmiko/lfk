package k8s

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/janosmiko/lfk/internal/model"
)

// GetFieldOwners fetches one object and returns which field manager last
// wrote each of its paths. This is a separate call from GetResourceYAML on
// purpose: that path strips managedFields so the rendered YAML stays free of
// the noise, and the gutter is loaded only when the user asks for it.
// Virtual resource types have no API object, so they return empty owners.
func (c *Client) GetFieldOwners(
	ctx context.Context, contextName, namespace string, rt model.ResourceTypeEntry, name string,
) (*FieldOwners, error) {
	if strings.HasPrefix(rt.APIGroup, "_") {
		return NewFieldOwners(nil), nil
	}

	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return nil, err
	}

	gvr := schema.GroupVersionResource{Group: rt.APIGroup, Version: rt.APIVersion, Resource: rt.Resource}
	var getter dynamic.ResourceInterface = dynClient.Resource(gvr)
	if rt.Namespaced {
		getter = dynClient.Resource(gvr).Namespace(namespace)
	}

	obj, err := getter.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting field owners: %w", err)
	}
	return NewFieldOwners(obj.GetManagedFields()), nil
}

// FieldOwner is the field manager that last wrote one path of an object.
type FieldOwner struct {
	Manager     string
	Operation   string
	Subresource string
	Time        time.Time
}

// PathSeg is one step of a path into an object. Key names a map field. Item
// holds the scalar fields of a list element, which is how managedFields
// identifies list entries: it stores a selector such as k:{"name":"nginx"}
// rather than an index, because indexes move.
type PathSeg struct {
	Key  string
	Item map[string]string
}

// FieldOwners answers "which manager last wrote this path" for one object,
// built from .metadata.managedFields. Ownership is recorded per path, so a
// parent is owned only when a manager wrote the parent itself.
type FieldOwners struct {
	root     *ownerNode
	managers []string
}

type ownerNode struct {
	owner  *FieldOwner
	fields map[string]*ownerNode

	// items is indexed twice over: first by which fields the selector names
	// (almost always "name"), then by those field values joined. A linear
	// scan here would be O(list length) per line, which squares on a large
	// tracked list in a big CRD.
	items map[string]map[string]*ownerNode
}

// NewFieldOwners builds the lookup from the managedFields of one object.
// Entries without usable FieldsV1 data are skipped: a malformed entry must
// not invent ownership.
func NewFieldOwners(entries []metav1.ManagedFieldsEntry) *FieldOwners {
	f := &FieldOwners{root: newOwnerNode()}
	seen := make(map[string]struct{}, len(entries))
	for i := range entries {
		e := &entries[i]
		if e.FieldsV1 == nil || e.Manager == "" {
			continue
		}
		owner := FieldOwner{
			Manager:     e.Manager,
			Operation:   string(e.Operation),
			Subresource: e.Subresource,
		}
		if e.Time != nil {
			owner.Time = e.Time.Time
		}
		if err := f.root.merge(e.FieldsV1.GetRawBytes(), owner, 0); err != nil {
			continue
		}
		if _, ok := seen[e.Manager]; !ok {
			seen[e.Manager] = struct{}{}
			f.managers = append(f.managers, e.Manager)
		}
	}
	slices.Sort(f.managers)
	return f
}

// Empty reports whether there is no ownership information at all.
func (f *FieldOwners) Empty() bool {
	return f == nil || f.root == nil || (len(f.root.fields) == 0 && len(f.root.items) == 0)
}

// Managers returns the distinct field managers, sorted. The order is stable
// so a color assigned to a manager does not move between renders.
func (f *FieldOwners) Managers() []string {
	if f == nil {
		return nil
	}
	return f.managers
}

// At returns the owner of an exact path. A path with no manager of its own
// returns false; the caller decides whether to inherit from an ancestor.
func (f *FieldOwners) At(path []PathSeg) (FieldOwner, bool) {
	if f == nil || f.root == nil {
		return FieldOwner{}, false
	}
	node := f.root
	for _, seg := range path {
		if node = node.child(seg); node == nil {
			return FieldOwner{}, false
		}
	}
	if node.owner == nil {
		return FieldOwner{}, false
	}
	return *node.owner, true
}

func newOwnerNode() *ownerNode {
	return &ownerNode{fields: make(map[string]*ownerNode)}
}

// selectorKeyNames returns the sorted field names of a selector, which is how
// the item index is bucketed.
func selectorKeyNames(sel map[string]string) string {
	names := make([]string, 0, len(sel))
	for k := range sel {
		names = append(names, k)
	}
	slices.Sort(names)
	return strings.Join(names, "\x00")
}

// selectorValueKey joins the values of the named fields, or reports false when
// the item does not carry all of them.
func selectorValueKey(names string, fields map[string]string) (string, bool) {
	var b strings.Builder
	for i, name := range strings.Split(names, "\x00") {
		value, ok := fields[name]
		if !ok {
			return "", false
		}
		if i > 0 {
			b.WriteByte(0)
		}
		b.WriteString(value)
	}
	return b.String(), true
}

func (n *ownerNode) child(seg PathSeg) *ownerNode {
	if seg.Item != nil {
		return n.matchItem(seg.Item)
	}
	return n.fields[seg.Key]
}

// matchItem finds the list entry whose selector the YAML item satisfies. A
// selector matches when every field it names is present with the same value.
func (n *ownerNode) matchItem(fields map[string]string) *ownerNode {
	for names, byValue := range n.items {
		key, ok := selectorValueKey(names, fields)
		if !ok {
			continue
		}
		if node, found := byValue[key]; found {
			return node
		}
	}
	return nil
}

// merge folds one managedFields entry into the tree. Keys follow the FieldsV1
// encoding: "f:" a map field, "k:" a list entry selector, "." the node itself.
// "v:" and "i:" entries are skipped, because a value or index selector has no
// stable anchor in the rendered YAML.
func (n *ownerNode) merge(raw []byte, owner FieldOwner, depth int) error {
	if depth > maxFieldsV1Depth {
		return errFieldsV1TooDeep
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	for key, value := range fields {
		switch {
		case key == ".":
			n.setOwner(owner)
		case strings.HasPrefix(key, "f:"):
			child := n.field(strings.TrimPrefix(key, "f:"))
			n.mergeChild(child, value, owner, depth+1)
		case strings.HasPrefix(key, "k:"):
			sel, err := parseSelector(strings.TrimPrefix(key, "k:"))
			if err != nil {
				continue
			}
			n.mergeChild(n.item(sel), value, owner, depth+1)
		}
	}
	return nil
}

// mergeChild owns the child when the value is an empty object, which is how
// FieldsV1 marks a leaf, and otherwise recurses.
func (n *ownerNode) mergeChild(child *ownerNode, value json.RawMessage, owner FieldOwner, depth int) {
	if isEmptyObject(value) {
		child.setOwner(owner)
		return
	}
	_ = child.merge(value, owner, depth)
}

func (n *ownerNode) field(name string) *ownerNode {
	if child, ok := n.fields[name]; ok {
		return child
	}
	child := newOwnerNode()
	n.fields[name] = child
	return child
}

func (n *ownerNode) item(sel map[string]string) *ownerNode {
	if len(sel) == 0 {
		return newOwnerNode()
	}
	names := selectorKeyNames(sel)
	key, _ := selectorValueKey(names, sel)
	if n.items == nil {
		n.items = make(map[string]map[string]*ownerNode, 1)
	}
	byValue, ok := n.items[names]
	if !ok {
		byValue = make(map[string]*ownerNode, 4)
		n.items[names] = byValue
	}
	if child, found := byValue[key]; found {
		return child
	}
	child := newOwnerNode()
	byValue[key] = child
	return child
}

// setOwner keeps the most recent writer when two entries claim one path.
func (n *ownerNode) setOwner(owner FieldOwner) {
	if n.owner != nil && !owner.Time.After(n.owner.Time) {
		return
	}
	n.owner = &owner
}

// parseSelector reads a k: selector such as {"name":"nginx"} or
// {"containerPort":80,"protocol":"TCP"}. Numbers keep their literal text so
// they compare equal to the value printed in the YAML.
func parseSelector(raw string) (map[string]string, error) {
	dec := json.NewDecoder(bytes.NewBufferString(raw))
	dec.UseNumber()
	var fields map[string]any
	if err := dec.Decode(&fields); err != nil {
		return nil, err
	}
	sel := make(map[string]string, len(fields))
	for k, v := range fields {
		switch t := v.(type) {
		case string:
			sel[k] = t
		case json.Number:
			sel[k] = t.String()
		case bool:
			if t {
				sel[k] = "true"
			} else {
				sel[k] = "false"
			}
		default:
			return nil, errUnsupportedSelector
		}
	}
	return sel, nil
}

var (
	errUnsupportedSelector = errors.New("unsupported managedFields selector value")
	errFieldsV1TooDeep     = errors.New("managedFields entry nests deeper than lfk walks")
)

// maxFieldsV1Depth bounds the walk. Real Kubernetes objects stay well under
// this; the cap stops a hostile or corrupt entry from driving the recursion.
const maxFieldsV1Depth = 32

func isEmptyObject(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("{}"))
}
