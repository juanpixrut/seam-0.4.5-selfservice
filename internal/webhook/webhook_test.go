package webhook

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestWebhook_MatchesEvent(t *testing.T) {
	svc := NewService(DefaultConfig())
	defer svc.Stop()

	tests := []struct {
		name       string
		events     []EventType
		checkEvent EventType
		want       bool
	}{
		{
			name:       "exact match",
			events:     []EventType{EventAssetCreated, EventAssetUpdated},
			checkEvent: EventAssetCreated,
			want:       true,
		},
		{
			name:       "wildcard",
			events:     []EventType{"*"},
			checkEvent: EventAssetDeleted,
			want:       true,
		},
		{
			name:       "no match",
			events:     []EventType{EventAssetCreated},
			checkEvent: EventAssetDeleted,
			want:       false,
		},
		{
			name:       "empty events",
			events:     []EventType{},
			checkEvent: EventAssetCreated,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			webhook := &Webhook{Events: tt.events}
			got := svc.matchesEvent(webhook, tt.checkEvent)
			if got != tt.want {
				t.Errorf("matchesEvent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestService_RegisterUnregister(t *testing.T) {
	svc := NewService(DefaultConfig())
	defer svc.Stop()

	webhook := &Webhook{
		ID:       1,
		TenantID: 100,
		Name:     "Test Webhook",
		URL:      "https://example.com/webhook",
		Active:   true,
		Events:   []EventType{EventAssetCreated},
	}

	// Register
	svc.Register(webhook)

	// Verify registered
	got, err := svc.GetWebhook(1)
	if err != nil {
		t.Fatalf("GetWebhook() error: %v", err)
	}
	if got.Name != webhook.Name {
		t.Errorf("Name = %q, want %q", got.Name, webhook.Name)
	}

	// Unregister
	svc.Unregister(1)

	// Verify unregistered
	_, err = svc.GetWebhook(1)
	if err != ErrWebhookNotFound {
		t.Errorf("GetWebhook() error = %v, want ErrWebhookNotFound", err)
	}
}

func TestService_GetWebhooks(t *testing.T) {
	svc := NewService(DefaultConfig())
	defer svc.Stop()

	// Register webhooks for different tenants
	svc.Register(&Webhook{ID: 1, TenantID: 100, Active: true})
	svc.Register(&Webhook{ID: 2, TenantID: 100, Active: true})
	svc.Register(&Webhook{ID: 3, TenantID: 200, Active: true})

	// Get webhooks for tenant 100
	webhooks := svc.GetWebhooks(100)
	if len(webhooks) != 2 {
		t.Errorf("GetWebhooks(100) returned %d webhooks, want 2", len(webhooks))
	}

	// Get webhooks for tenant 200
	webhooks = svc.GetWebhooks(200)
	if len(webhooks) != 1 {
		t.Errorf("GetWebhooks(200) returned %d webhooks, want 1", len(webhooks))
	}

	// Get webhooks for non-existent tenant
	webhooks = svc.GetWebhooks(999)
	if len(webhooks) != 0 {
		t.Errorf("GetWebhooks(999) returned %d webhooks, want 0", len(webhooks))
	}
}

func TestService_Trigger(t *testing.T) {
	var received int32
	var receivedPayload *Payload

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&received, 1)

		// Parse payload
		var p Payload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			t.Errorf("Failed to decode payload: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		receivedPayload = &p

		// Check signature header exists
		if r.Header.Get("X-Webhook-Signature") == "" {
			t.Error("Missing X-Webhook-Signature header")
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create service with fast delivery
	cfg := DefaultConfig()
	cfg.WorkerCount = 1
	svc := NewService(cfg)
	defer svc.Stop()

	// Register webhook
	svc.Register(&Webhook{
		ID:       1,
		TenantID: 100,
		URL:      server.URL,
		Secret:   "test-secret",
		Active:   true,
		Events:   []EventType{EventAssetCreated},
	})

	// Trigger event
	data := map[string]interface{}{
		"asset_id": 123,
		"name":     "Test Asset",
	}
	err := svc.Trigger(context.Background(), EventAssetCreated, 100, data)
	if err != nil {
		t.Fatalf("Trigger() error: %v", err)
	}

	// Wait for delivery
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&received) != 1 {
		t.Errorf("Received %d webhook calls, want 1", received)
	}

	if receivedPayload == nil {
		t.Fatal("No payload received")
	}

	if receivedPayload.Event != EventAssetCreated {
		t.Errorf("Event = %q, want %q", receivedPayload.Event, EventAssetCreated)
	}

	if receivedPayload.TenantID != 100 {
		t.Errorf("TenantID = %d, want 100", receivedPayload.TenantID)
	}
}

func TestService_TriggerInactiveWebhook(t *testing.T) {
	var received int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&received, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.WorkerCount = 1
	svc := NewService(cfg)
	defer svc.Stop()

	// Register inactive webhook
	svc.Register(&Webhook{
		ID:       1,
		TenantID: 100,
		URL:      server.URL,
		Active:   false, // Inactive
		Events:   []EventType{EventAssetCreated},
	})

	// Trigger event
	svc.Trigger(context.Background(), EventAssetCreated, 100, nil)

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&received) != 0 {
		t.Errorf("Received %d webhook calls, want 0 (webhook is inactive)", received)
	}
}

func TestService_TriggerWrongTenant(t *testing.T) {
	var received int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&received, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.WorkerCount = 1
	svc := NewService(cfg)
	defer svc.Stop()

	// Register webhook for tenant 100
	svc.Register(&Webhook{
		ID:       1,
		TenantID: 100,
		URL:      server.URL,
		Active:   true,
		Events:   []EventType{EventAssetCreated},
	})

	// Trigger event for different tenant
	svc.Trigger(context.Background(), EventAssetCreated, 200, nil)

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&received) != 0 {
		t.Errorf("Received %d webhook calls, want 0 (wrong tenant)", received)
	}
}

func TestVerifySignature(t *testing.T) {
	payload := []byte(`{"event":"asset.created","data":{}}`)
	secret := "my-secret-key"

	// Generate signature the same way the service does
	svc := NewService(DefaultConfig())
	signature := svc.sign(payload, secret)

	// Verify
	if !VerifySignature(payload, signature, secret) {
		t.Error("VerifySignature() returned false for valid signature")
	}

	// Verify with wrong secret
	if VerifySignature(payload, signature, "wrong-secret") {
		t.Error("VerifySignature() returned true for wrong secret")
	}

	// Verify with tampered payload
	if VerifySignature([]byte(`{"tampered":true}`), signature, secret) {
		t.Error("VerifySignature() returned true for tampered payload")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", cfg.MaxRetries)
	}

	if cfg.WorkerCount != 5 {
		t.Errorf("WorkerCount = %d, want 5", cfg.WorkerCount)
	}

	if cfg.DefaultTimeout != 30*time.Second {
		t.Errorf("DefaultTimeout = %v, want 30s", cfg.DefaultTimeout)
	}
}
