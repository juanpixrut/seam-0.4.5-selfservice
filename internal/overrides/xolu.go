package overrides

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ha1tch/seam-ui/internal/auth"
)

// XoluStore keeps the overrides in the i18n_override and schema_override
// entities. It uses the client interface the user store and the feature-state
// store use, which server.go already adapts from the real xolu client.
type XoluStore struct {
	client auth.XOLUClient
}

// NewXoluStore returns an XoluStore.
func NewXoluStore(client auth.XOLUClient) *XoluStore { return &XoluStore{client: client} }

func oqlQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

func (s *XoluStore) rows(ctx context.Context, query string) ([]map[string]interface{}, error) {
	res, err := s.client.OQL(ctx, query)
	if auth.EntityNotYetCreated(err) {
		// No override of this kind has ever been saved on this xolu: the
		// entity appears with its first row.
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, nil
	}
	return res.Data, nil
}

// TenantsWithOverrides implements Store.
func (s *XoluStore) TenantsWithOverrides(ctx context.Context) ([]int64, error) {
	seen := map[int64]bool{}
	for _, entity := range []string{EntityTranslation, EntitySchema} {
		rows, err := s.rows(ctx, "SELECT * FROM "+entity)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			if t := asInt64(r["tenant_id"]); t > 0 {
				seen[t] = true
			}
		}
	}
	return sortedTenants(seen), nil
}

func decodeTranslation(r map[string]interface{}) (Translation, bool) {
	locale, _ := r["locale"].(string)
	key, _ := r["key"].(string)
	if locale == "" || key == "" {
		return Translation{}, false
	}
	value, _ := r["value"].(string)
	return Translation{Tenant: asInt64(r["tenant_id"]), Locale: locale, Key: key, Value: value,
		ChangedBy: asInt64(r["changed_by"]), ChangedAt: asTime(r["changed_at"])}, true
}

// Translations implements Store.
func (s *XoluStore) Translations(ctx context.Context, tenant int64) ([]Translation, error) {
	if err := positive(tenant); err != nil {
		return nil, err
	}
	rows, err := s.rows(ctx, fmt.Sprintf("SELECT * FROM %s WHERE tenant_id = %d", EntityTranslation, tenant))
	if err != nil {
		return nil, err
	}
	var out []Translation
	for _, r := range rows {
		if t, ok := decodeTranslation(r); ok && t.Tenant == tenant {
			out = append(out, t)
		}
	}
	return out, nil
}

func (s *XoluStore) findTranslation(ctx context.Context, tenant int64, locale, key string) (int64, bool, error) {
	rows, err := s.rows(ctx, fmt.Sprintf("SELECT TOP 1 * FROM %s WHERE tenant_id = %d AND locale = %s AND key = %s",
		EntityTranslation, tenant, oqlQuote(locale), oqlQuote(key)))
	if err != nil || len(rows) == 0 {
		return 0, false, err
	}
	return asInt64(rows[0]["id"]), true, nil
}

// SetTranslation implements Store: update the row if there is one, else create it.
func (s *XoluStore) SetTranslation(ctx context.Context, t Translation) error {
	if err := positive(t.Tenant); err != nil {
		return err
	}
	data := map[string]interface{}{
		"tenant_id": t.Tenant, "locale": t.Locale, "key": t.Key, "value": t.Value,
		"changed_by": t.ChangedBy, "changed_at": t.ChangedAt.UTC().Format(time.RFC3339),
	}
	id, found, err := s.findTranslation(ctx, t.Tenant, t.Locale, t.Key)
	if err != nil {
		return err
	}
	if found {
		_, err = s.client.Update(ctx, EntityTranslation, id, data)
		return err
	}
	_, err = s.client.Create(ctx, EntityTranslation, data)
	return err
}

// RemoveTranslation implements Store.
func (s *XoluStore) RemoveTranslation(ctx context.Context, tenant int64, locale, key string) error {
	if err := positive(tenant); err != nil {
		return err
	}
	id, found, err := s.findTranslation(ctx, tenant, locale, key)
	if err != nil || !found {
		return err
	}
	return s.client.Delete(ctx, EntityTranslation, id)
}

// TranslationHistory implements Store.
func (s *XoluStore) TranslationHistory(ctx context.Context, tenant int64, locale, key string) ([]TranslationVersion, error) {
	if err := positive(tenant); err != nil {
		return nil, err
	}
	rows, err := s.rows(ctx, fmt.Sprintf("SELECT * FROM %s WHERE tenant_id = %d AND locale = %s AND key = %s",
		EntityTranslationHistory, tenant, oqlQuote(locale), oqlQuote(key)))
	if err != nil {
		return nil, err
	}
	var out []TranslationVersion
	for _, r := range rows {
		l, _ := r["locale"].(string)
		k, _ := r["key"].(string)
		if asInt64(r["tenant_id"]) != tenant || l != locale || k != key {
			continue
		}
		value, _ := r["value"].(string)
		out = append(out, TranslationVersion{Tenant: tenant, Locale: l, Key: k, Version: int(asInt64(r["version"])), Value: value,
			Removed: asBool(r["removed"]), ChangedBy: asInt64(r["changed_by"]), ChangedAt: asTime(r["changed_at"])})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Version != out[j].Version {
			return out[i].Version > out[j].Version
		}
		return out[i].ChangedAt.After(out[j].ChangedAt)
	})
	return out, nil
}

// AppendTranslationVersion implements Store.
func (s *XoluStore) AppendTranslationVersion(ctx context.Context, v TranslationVersion) (TranslationVersion, error) {
	if err := positive(v.Tenant); err != nil {
		return TranslationVersion{}, err
	}
	history, err := s.TranslationHistory(ctx, v.Tenant, v.Locale, v.Key)
	if err != nil {
		return TranslationVersion{}, err
	}
	v.Version = 1
	if len(history) > 0 {
		v.Version = history[0].Version + 1
	}
	_, err = s.client.Create(ctx, EntityTranslationHistory, map[string]interface{}{
		"tenant_id": v.Tenant, "locale": v.Locale, "key": v.Key, "version": v.Version,
		"value": v.Value, "removed": v.Removed,
		"changed_by": v.ChangedBy, "changed_at": v.ChangedAt.UTC().Format(time.RFC3339),
	})
	if err != nil {
		return TranslationVersion{}, err
	}
	return v, nil
}

func decodeSchema(r map[string]interface{}) (SchemaVersion, bool) {
	module, _ := r["module"].(string)
	name, _ := r["name"].(string)
	if module == "" || name == "" {
		return SchemaVersion{}, false
	}
	doc, _ := r["doc"].(string)
	note, _ := r["note"].(string)
	return SchemaVersion{Tenant: asInt64(r["tenant_id"]), Module: module, Name: name, Version: int(asInt64(r["version"])),
		Doc: []byte(doc), Removed: asBool(r["removed"]), Note: note,
		ChangedBy: asInt64(r["changed_by"]), ChangedAt: asTime(r["changed_at"])}, true
}

func (s *XoluStore) versions(ctx context.Context, query string, tenant int64) ([]SchemaVersion, error) {
	rows, err := s.rows(ctx, query)
	if err != nil {
		return nil, err
	}
	var out []SchemaVersion
	for _, r := range rows {
		if v, ok := decodeSchema(r); ok && v.Tenant == tenant {
			out = append(out, v)
		}
	}
	return out, nil
}

// newestFirst orders versions by version number, then by time, newest first.
func newestFirst(vs []SchemaVersion) {
	sort.SliceStable(vs, func(i, j int) bool {
		if vs[i].Version != vs[j].Version {
			return vs[i].Version > vs[j].Version
		}
		return vs[i].ChangedAt.After(vs[j].ChangedAt)
	})
}

// CurrentSchemas implements Store.
func (s *XoluStore) CurrentSchemas(ctx context.Context, tenant int64) ([]SchemaVersion, error) {
	if err := positive(tenant); err != nil {
		return nil, err
	}
	all, err := s.versions(ctx, fmt.Sprintf("SELECT * FROM %s WHERE tenant_id = %d", EntitySchema, tenant), tenant)
	if err != nil {
		return nil, err
	}
	newestFirst(all)
	seen := map[[2]string]bool{}
	var out []SchemaVersion
	for _, v := range all {
		k := [2]string{v.Module, v.Name}
		if seen[k] {
			continue
		}
		seen[k] = true
		if !v.Removed {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Module != out[j].Module {
			return out[i].Module < out[j].Module
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// SchemaHistory implements Store.
func (s *XoluStore) SchemaHistory(ctx context.Context, tenant int64, module, name string) ([]SchemaVersion, error) {
	if err := positive(tenant); err != nil {
		return nil, err
	}
	vs, err := s.versions(ctx, fmt.Sprintf("SELECT * FROM %s WHERE tenant_id = %d AND module = %s AND name = %s",
		EntitySchema, tenant, oqlQuote(module), oqlQuote(name)), tenant)
	if err != nil {
		return nil, err
	}
	newestFirst(vs)
	return vs, nil
}

// AppendSchema implements Store.
func (s *XoluStore) AppendSchema(ctx context.Context, v SchemaVersion) (SchemaVersion, error) {
	if err := positive(v.Tenant); err != nil {
		return SchemaVersion{}, err
	}
	history, err := s.SchemaHistory(ctx, v.Tenant, v.Module, v.Name)
	if err != nil {
		return SchemaVersion{}, err
	}
	v.Version = 1
	if len(history) > 0 {
		v.Version = history[0].Version + 1
	}
	_, err = s.client.Create(ctx, EntitySchema, map[string]interface{}{
		"tenant_id": v.Tenant, "module": v.Module, "name": v.Name, "version": v.Version,
		"doc": string(v.Doc), "removed": v.Removed, "note": v.Note,
		"changed_by": v.ChangedBy, "changed_at": v.ChangedAt.UTC().Format(time.RFC3339),
	})
	if err != nil {
		return SchemaVersion{}, err
	}
	return v, nil
}

// Fingerprint implements Store: a hash over the identity, time and version of
// every row of both entities, so any added, changed or deleted row changes it.
func (s *XoluStore) Fingerprint(ctx context.Context) (string, error) {
	var lines []string
	for _, entity := range []string{EntityTranslation, EntitySchema} {
		rows, err := s.rows(ctx, "SELECT * FROM "+entity)
		if err != nil {
			return "", err
		}
		for _, r := range rows {
			lines = append(lines, fmt.Sprintf("%s|%d|%d|%d|%s|%s", entity, asInt64(r["id"]), asInt64(r["tenant_id"]), asInt64(r["version"]),
				fmt.Sprint(r["changed_at"]), fmt.Sprint(r["value"])+fmt.Sprint(r["removed"])))
		}
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:]), nil
}

func asInt64(v interface{}) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	case json.Number:
		i, _ := n.Int64()
		return i
	case string:
		i, _ := strconv.ParseInt(n, 10, 64)
		return i
	}
	return 0
}

func asBool(v interface{}) bool {
	switch b := v.(type) {
	case bool:
		return b
	case string:
		return b == "true" || b == "1"
	case float64:
		return b != 0
	}
	return false
}

func asTime(v interface{}) time.Time {
	if s, _ := v.(string); s != "" {
		t, _ := time.Parse(time.RFC3339, s)
		return t
	}
	return time.Time{}
}
