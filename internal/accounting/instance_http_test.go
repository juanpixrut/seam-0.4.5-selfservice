package accounting

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPInstancePullAndMark(t *testing.T) {
	var marked, auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		switch {
		case r.Method == "GET" && r.URL.Path == "/seam-admin/v1/snapshot":
			w.Write([]byte(`{"tenants":[{"external_ref":"t1","name":"Acme","country":"PY","ae_ref":"seam_web"}],
			"items":[{"external_ref":"i 1","tenant_ref":"t1","period":"2026-09","concept":"subscription","amount":"1,234.50","state":"pending"}]}`))
		case r.Method == "POST" && strings.HasSuffix(r.URL.EscapedPath(), "/collected"):
			marked = r.URL.EscapedPath()
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	c := NewHTTPInstance(srv.URL+"/", "k3y", time.Second)
	snap, err := c.Pull(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Items) != 1 || snap.Items[0].Amount != 123450 || snap.Tenants[0].AERef != "seam_web" {
		t.Fatalf("snapshot = %+v", snap)
	}
	if auth != "Bearer k3y" {
		t.Fatalf("auth header %q", auth)
	}
	if err := c.MarkCollected(context.Background(), "i 1"); err != nil || marked != "/seam-admin/v1/items/i%201/collected" {
		t.Fatalf("mark: %v %q", err, marked)
	}
}

func TestHTTPInstanceRejectsBadData(t *testing.T) {
	for name, body := range map[string]string{
		"collected state": `{"items":[{"external_ref":"a","amount":"1.00","state":"collected"}]}`,
		"float amount":    `{"items":[{"external_ref":"a","amount":"1.005","state":"pending"}]}`,
		"not json":        `<html>`,
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
		if _, err := NewHTTPInstance(srv.URL, "k", time.Second).Pull(context.Background()); err == nil {
			t.Errorf("%s: want an error", name)
		}
		srv.Close()
	}
}

func TestHTTPInstanceDoesNotFollowRedirectsOrAcceptErrors(t *testing.T) {
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("redirect was followed with the key")
	}))
	defer other.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL, http.StatusFound)
	}))
	defer srv.Close()
	if _, err := NewHTTPInstance(srv.URL, "k", time.Second).Pull(context.Background()); err == nil {
		t.Fatal("a redirect must be an error")
	}
	if err := NewHTTPInstance(srv.URL, "k", time.Second).MarkCollected(context.Background(), ""); err == nil {
		t.Fatal("empty ref must fail")
	}
}

func TestRateCalculator(t *testing.T) {
	r := RateCalculator{RealBPS: 1000, SeamWebBPS: 250}
	real := AE{Kind: AEReal}
	web := AE{Kind: AEVirtual, Slug: SlugSeamWeb}
	ae := AE{Kind: AEVirtual, Slug: SlugSeamAE}
	got := func(a AE, amt Cents) []CommissionResult {
		out, _ := r.Calculate(context.Background(), Item{Amount: amt}, Tenant{}, a)
		return out
	}
	if g := got(real, 100000); len(g) != 1 || g[0].Amount != 10000 || g[0].Detail != "10%" {
		t.Fatalf("real: %+v", g)
	}
	if g := got(web, 10001); len(g) != 1 || g[0].Amount != 250 || g[0].Detail != "2.5%" { // 250.025 rounds to 250
		t.Fatalf("web: %+v", g)
	}
	if got(ae, 100000) != nil {
		t.Fatal("zero rate must accrue nothing")
	}
	if got(real, 3) != nil { // 0.3 cents rounds to 0
		t.Fatal("sub-cent commission must be dropped")
	}
	if (RateCalculator{}).Enabled() || !r.Enabled() {
		t.Fatal("Enabled")
	}
}
