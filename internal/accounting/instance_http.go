package accounting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HTTPInstance is an InstanceClient over a small JSON API. The contract (also
// in docs/ACCOUNTING_ADMIN_MODULE.md), all calls with "Authorization: Bearer
// <key>":
//
//	GET  {base}/seam-admin/v1/snapshot
//	     -> {"tenants":[{"external_ref","name","country","ae_ref","account","registration"}],
//	         "items":[{"external_ref","tenant_ref","period":"YYYY-MM","concept",
//	                   "description","amount":"123.45","state":"pending|collection_in_progress"}]}
//	POST {base}/seam-admin/v1/items/{external_ref}/collected
//	     -> 2xx; must be idempotent (a repeat for a collected item is a 2xx too)
//
// Amounts are decimal strings in USD, never floats. Redirects are not
// followed: the key must not travel to a host nobody configured.
type HTTPInstance struct {
	BaseURL string
	APIKey  string
	Client  *http.Client
}

const maxSnapshotBytes = 32 << 20

// NewHTTPInstance returns a client for base with the given key and timeout.
func NewHTTPInstance(base, key string, timeout time.Duration) *HTTPInstance {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &HTTPInstance{
		BaseURL: strings.TrimRight(base, "/"),
		APIKey:  key,
		Client: &http.Client{
			Timeout:       timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

func (c *HTTPInstance) do(ctx context.Context, method, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Accept", "application/json")
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("accounting: instance unreachable: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		resp.Body.Close()
		return nil, fmt.Errorf("accounting: instance answered %d", resp.StatusCode)
	}
	return resp, nil
}

type wireSnapshot struct {
	Tenants []struct {
		ExternalRef  string `json:"external_ref"`
		Name         string `json:"name"`
		Country      string `json:"country"`
		AERef        string `json:"ae_ref"`
		Account      string `json:"account"`
		Registration string `json:"registration"`
	} `json:"tenants"`
	Items []struct {
		ExternalRef string `json:"external_ref"`
		TenantRef   string `json:"tenant_ref"`
		Period      string `json:"period"`
		Concept     string `json:"concept"`
		Description string `json:"description"`
		Amount      string `json:"amount"`
		State       string `json:"state"`
	} `json:"items"`
}

// Pull implements InstanceClient. A malformed record fails the whole pull: a
// half-read snapshot must not be mistaken for a complete one.
func (c *HTTPInstance) Pull(ctx context.Context) (InstanceSnapshot, error) {
	resp, err := c.do(ctx, http.MethodGet, "/seam-admin/v1/snapshot")
	if err != nil {
		return InstanceSnapshot{}, err
	}
	defer resp.Body.Close()
	var w wireSnapshot
	lr := io.LimitReader(resp.Body, maxSnapshotBytes+1)
	if err := json.NewDecoder(lr).Decode(&w); err != nil {
		return InstanceSnapshot{}, fmt.Errorf("accounting: instance snapshot is not valid JSON: %w", err)
	}
	var out InstanceSnapshot
	for _, t := range w.Tenants {
		out.Tenants = append(out.Tenants, TenantSnapshot{ExternalRef: t.ExternalRef, Name: t.Name,
			Country: t.Country, AERef: t.AERef, Account: t.Account, Registration: t.Registration})
	}
	for i, it := range w.Items {
		amt, err := ParseUSD(it.Amount)
		if err != nil {
			return InstanceSnapshot{}, fmt.Errorf("accounting: snapshot item %d (%q): %w", i, it.ExternalRef, err)
		}
		st := ItemState(it.State)
		if st != ItemPending && st != ItemInProgress {
			return InstanceSnapshot{}, fmt.Errorf("accounting: snapshot item %q has state %q; the instance may only report pending or collection_in_progress", it.ExternalRef, it.State)
		}
		out.Items = append(out.Items, ItemSnapshot{ExternalRef: it.ExternalRef, TenantRef: it.TenantRef,
			Period: it.Period, Concept: Concept(it.Concept), Description: it.Description, Amount: amt, State: st})
	}
	return out, nil
}

// MarkCollected implements InstanceClient.
func (c *HTTPInstance) MarkCollected(ctx context.Context, ref string) error {
	if strings.TrimSpace(ref) == "" {
		return errors.New("accounting: empty item reference")
	}
	resp, err := c.do(ctx, http.MethodPost, "/seam-admin/v1/items/"+url.PathEscape(ref)+"/collected")
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
