package ui

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sync"
)

// Which translation keys a form's page asks for.
//
// The engine builds a field's label key from the module and form the page
// passes when it renders (FORM_ENGINE_SPEC section 4.1), and those are not
// always the definition's package and file name. So the tools do not guess
// once they can know: resolveSchema notes which definition it handed out, the
// engine reports each render with the identity it used (fe.RenderObserver),
// and the two are matched by the definition's bytes. From then on the editors
// name a form's label keys with the page's own identity. Before the page has
// been seen they fall back to the naming conventions (developerLabelKeys) and
// say so.
//
// What is learnt is a property of the code, not of a tenant or a session. So
// the build carries what is already known (developer_pages_seed.json: the
// identities seen by opening every form page on a live stack, and the ones the
// source states as literals, which a test keeps in step with the code), and
// what a running instance learns is kept across restarts where there is
// somewhere to keep it: a small file beside the override directories' history
// (SetDeveloperTraceFile), which wins over the seed, being the later
// observation. A page that changes its identity in a later build corrects its
// entry the next time it is rendered. A form in neither is named by the
// conventions until its page is opened.
//
// The tools' own renders (previews, the designer, the try page) are marked
// with withoutTrace, so a preview never passes for the page.

type developerRenderIdentity struct{ Module, Form string }

type developerLabelTrace struct {
	mu   sync.Mutex
	docs map[[sha256.Size]byte]developerSchemaRef
	// a form rendered by more than one page can have more than one identity
	// (the two steps of a work-order template); the newest is last
	pairs map[developerSchemaRef][]developerRenderIdentity
	path  string // where the identities are kept, "" for nowhere
}

// developerTraceMaxIdentities bounds the identities kept for one form.
const developerTraceMaxIdentities = 8

// SetDeveloperTraceFile names the file the identities learnt are kept in, and
// reads what it already holds; "" keeps them in memory only. The server calls
// it once, with a file under an override directory's .history, when one is
// configured.
func SetDeveloperTraceFile(path string) { developerTrace.setFile(path) }

func (t *developerLabelTrace) setFile(path string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.path = path
	if path == "" {
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	// a record, not a source: an unreadable one is rewritten at the next render
	t.mergeLocked(raw)
}

var builtInFormNameRE = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

func cutRef(s string) (module, name string, ok bool) {
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			// built-in forms may have a hyphen in their name (workorders/step-2)
			return s[:i], s[i+1:], developerNameRE.MatchString(s[:i]) && builtInFormNameRE.MatchString(s[i+1:])
		}
	}
	return "", "", false
}

// saveLocked writes the identities to the file, if there is one; the caller
// holds the lock. A failure is not reported: the table in memory is right, and
// the next change writes it again.
func (t *developerLabelTrace) saveLocked() {
	if t.path == "" {
		return
	}
	stored := make(map[string][]developerRenderIdentity, len(t.pairs))
	for ref, ids := range t.pairs {
		stored[ref.String()] = ids
	}
	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return
	}
	if os.MkdirAll(filepath.Dir(t.path), 0o755) != nil {
		return
	}
	tmp := t.path + ".tmp"
	if os.WriteFile(tmp, append(data, '\n'), 0o644) != nil {
		return
	}
	if os.Rename(tmp, t.path) != nil {
		_ = os.Remove(tmp)
	}
}

// developerTraceMaxDocs bounds the definitions remembered; when it is reached
// the table starts again (the identities already learnt are kept).
const developerTraceMaxDocs = 4096

var developerTrace = func() *developerLabelTrace {
	t := &developerLabelTrace{docs: map[[sha256.Size]byte]developerSchemaRef{}, pairs: map[developerSchemaRef][]developerRenderIdentity{}}
	t.mergeLocked(developerPagesSeed)
	return t
}()

//go:embed developer_pages_seed.json
var developerPagesSeed []byte

// mergeLocked adds the identities of a stored table, replacing the ones it
// names; a form's entry is one identity or a list of them. A table that
// cannot be read teaches nothing.
func (t *developerLabelTrace) mergeLocked(raw []byte) {
	var stored map[string]json.RawMessage
	if json.Unmarshal(raw, &stored) != nil {
		return
	}
	for name, entry := range stored {
		module, form, ok := cutRef(name)
		if !ok {
			continue
		}
		var ids []developerRenderIdentity
		if json.Unmarshal(entry, &ids) != nil {
			var one developerRenderIdentity
			if json.Unmarshal(entry, &one) != nil {
				continue
			}
			ids = []developerRenderIdentity{one}
		}
		var good []developerRenderIdentity
		for _, id := range ids {
			if id.Module != "" && id.Form != "" && len(good) < developerTraceMaxIdentities {
				good = append(good, id)
			}
		}
		if len(good) > 0 {
			t.pairs[developerSchemaRef{module, form}] = good
		}
	}
}

// reseed forgets what was learnt and starts again from the build's table
// (tests).
func (t *developerLabelTrace) reseed() {
	t.reset()
	t.mu.Lock()
	defer t.mu.Unlock()
	t.mergeLocked(developerPagesSeed)
}

type developerNoTraceKey struct{}

// withoutTrace marks a context whose renders are the tools' own.
func withoutTrace(ctx context.Context) context.Context {
	return context.WithValue(ctx, developerNoTraceKey{}, true)
}

// noteDoc remembers that doc is the definition of module/name in force for
// some request.
func (t *developerLabelTrace) noteDoc(module, name string, doc []byte) {
	if len(doc) == 0 {
		return
	}
	sum := sha256.Sum256(doc)
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.docs) >= developerTraceMaxDocs {
		t.docs = map[[sha256.Size]byte]developerSchemaRef{}
	}
	t.docs[sum] = developerSchemaRef{module, name}
}

// observe is the engine's render observer.
func (t *developerLabelTrace) observe(ctx context.Context, schema []byte, module, form string) {
	if module == "" || form == "" || ctx.Value(developerNoTraceKey{}) != nil {
		return
	}
	sum := sha256.Sum256(schema)
	t.mu.Lock()
	defer t.mu.Unlock()
	if ref, ok := t.docs[sum]; ok {
		id := developerRenderIdentity{module, form}
		for _, known := range t.pairs[ref] {
			if known == id {
				return
			}
		}
		ids := append(t.pairs[ref], id)
		if len(ids) > developerTraceMaxIdentities {
			ids = ids[len(ids)-developerTraceMaxIdentities:]
		}
		t.pairs[ref] = ids
		t.saveLocked()
	}
}

// identity is the i18n identity the form's page was last seen rendering with.
func (t *developerLabelTrace) identity(ref developerSchemaRef) (developerRenderIdentity, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	ids := t.pairs[ref]
	if len(ids) == 0 {
		return developerRenderIdentity{}, false
	}
	id, ok := ids[len(ids)-1], true
	return id, ok
}

// reset forgets everything (tests).
func (t *developerLabelTrace) reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.docs = map[[sha256.Size]byte]developerSchemaRef{}
	t.pairs = map[developerSchemaRef][]developerRenderIdentity{}
	t.path = ""
}

// identities are all the identities the form's pages were seen rendering
// with, oldest first.
func (t *developerLabelTrace) identities(ref developerSchemaRef) []developerRenderIdentity {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]developerRenderIdentity(nil), t.pairs[ref]...)
}
