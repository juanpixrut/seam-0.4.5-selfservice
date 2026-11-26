// Package webhook provides outgoing webhook delivery for Seam AMS.
package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Common errors
var (
	ErrWebhookNotFound   = errors.New("webhook not found")
	ErrWebhookDisabled   = errors.New("webhook disabled")
	ErrDeliveryFailed    = errors.New("webhook delivery failed")
	ErrMaxRetriesReached = errors.New("maximum retries reached")
)

// EventType represents webhook event types.
type EventType string

const (
	EventAssetCreated       EventType = "asset.created"
	EventAssetUpdated       EventType = "asset.updated"
	EventAssetDeleted       EventType = "asset.deleted"
	EventAssetStatusChanged EventType = "asset.status_changed"
	EventEventIngested      EventType = "event.ingested"
	EventRuleTriggered      EventType = "rule.triggered"
	EventAlertCreated       EventType = "alert.created"
)

// Webhook represents a webhook configuration.
type Webhook struct {
	ID         int64             `json:"id"`
	TenantID   int64             `json:"tenant_id"`
	Name       string            `json:"name"`
	URL        string            `json:"url"`
	Secret     string            `json:"secret,omitempty"`
	Events     []EventType       `json:"events"`
	Active     bool              `json:"active"`
	Headers    map[string]string `json:"headers,omitempty"`
	RetryCount int               `json:"retry_count"`
	Timeout    int               `json:"timeout_seconds"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

// Delivery represents a webhook delivery attempt.
type Delivery struct {
	ID          int64     `json:"id"`
	WebhookID   int64     `json:"webhook_id"`
	EventType   EventType `json:"event_type"`
	Payload     string    `json:"payload"`
	StatusCode  int       `json:"status_code"`
	Response    string    `json:"response"`
	Attempts    int       `json:"attempts"`
	Success     bool      `json:"success"`
	Error       string    `json:"error,omitempty"`
	DeliveredAt time.Time `json:"delivered_at"`
	NextRetryAt time.Time `json:"next_retry_at,omitempty"`
}

// Payload represents the webhook payload sent to subscribers.
type Payload struct {
	ID        string      `json:"id"`
	Event     EventType   `json:"event"`
	TenantID  int64       `json:"tenant_id"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// Config holds webhook service configuration.
type Config struct {
	MaxRetries     int
	RetryBackoff   time.Duration
	DefaultTimeout time.Duration
	MaxPayloadSize int64
	WorkerCount    int
}

// DefaultConfig returns default webhook configuration.
func DefaultConfig() Config {
	return Config{
		MaxRetries:     3,
		RetryBackoff:   time.Minute,
		DefaultTimeout: 30 * time.Second,
		MaxPayloadSize: 1 << 20, // 1MB
		WorkerCount:    5,
	}
}

// Service provides webhook management and delivery.
type Service struct {
	config     Config
	client     *http.Client
	webhooks   map[int64]*Webhook
	mu         sync.RWMutex
	deliveries chan *deliveryJob
	stop       chan struct{}
}

type deliveryJob struct {
	webhook *Webhook
	payload *Payload
}

// NewService creates a new webhook service.
func NewService(cfg Config) *Service {
	s := &Service{
		config:     cfg,
		webhooks:   make(map[int64]*Webhook),
		deliveries: make(chan *deliveryJob, 1000),
		stop:       make(chan struct{}),
		client: &http.Client{
			Timeout: cfg.DefaultTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}

	// Start delivery workers
	for i := 0; i < cfg.WorkerCount; i++ {
		go s.deliveryWorker()
	}

	return s
}

// Register adds a webhook to the service.
func (s *Service) Register(webhook *Webhook) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.webhooks[webhook.ID] = webhook
}

// Unregister removes a webhook from the service.
func (s *Service) Unregister(id int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.webhooks, id)
}

// Trigger sends a webhook event to all matching subscribers.
func (s *Service) Trigger(ctx context.Context, eventType EventType, tenantID int64, data interface{}) error {
	payload := &Payload{
		ID:        generateID(),
		Event:     eventType,
		TenantID:  tenantID,
		Timestamp: time.Now(),
		Data:      data,
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, webhook := range s.webhooks {
		if !webhook.Active {
			continue
		}
		if webhook.TenantID != tenantID {
			continue
		}
		if !s.matchesEvent(webhook, eventType) {
			continue
		}

		// Queue for delivery
		select {
		case s.deliveries <- &deliveryJob{webhook: webhook, payload: payload}:
		default:
			// Queue full, log and skip
		}
	}

	return nil
}

// matchesEvent checks if a webhook subscribes to an event type.
func (s *Service) matchesEvent(webhook *Webhook, eventType EventType) bool {
	for _, e := range webhook.Events {
		if e == eventType || e == "*" {
			return true
		}
	}
	return false
}

// deliveryWorker processes webhook deliveries.
func (s *Service) deliveryWorker() {
	for {
		select {
		case <-s.stop:
			return
		case job := <-s.deliveries:
			s.deliver(job.webhook, job.payload)
		}
	}
}

// deliver sends a webhook payload with retries.
func (s *Service) deliver(webhook *Webhook, payload *Payload) *Delivery {
	delivery := &Delivery{
		WebhookID:   webhook.ID,
		EventType:   payload.Event,
		DeliveredAt: time.Now(),
	}

	// Serialize payload
	body, err := json.Marshal(payload)
	if err != nil {
		delivery.Error = fmt.Sprintf("marshal error: %v", err)
		return delivery
	}
	delivery.Payload = string(body)

	// Attempt delivery with retries
	maxAttempts := webhook.RetryCount
	if maxAttempts == 0 {
		maxAttempts = s.config.MaxRetries
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		delivery.Attempts = attempt

		statusCode, response, err := s.sendRequest(webhook, body)
		delivery.StatusCode = statusCode
		delivery.Response = response

		if err == nil && statusCode >= 200 && statusCode < 300 {
			delivery.Success = true
			return delivery
		}

		if err != nil {
			delivery.Error = err.Error()
		} else {
			delivery.Error = fmt.Sprintf("HTTP %d", statusCode)
		}

		// Wait before retry
		if attempt < maxAttempts {
			backoff := s.config.RetryBackoff * time.Duration(attempt)
			delivery.NextRetryAt = time.Now().Add(backoff)
			time.Sleep(backoff)
		}
	}

	return delivery
}

// sendRequest sends a single HTTP request.
func (s *Service) sendRequest(webhook *Webhook, body []byte) (int, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.client.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", webhook.URL, bytes.NewReader(body))
	if err != nil {
		return 0, "", fmt.Errorf("creating request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Seam-AMS-Webhook/1.0")

	// Custom headers
	for k, v := range webhook.Headers {
		req.Header.Set(k, v)
	}

	// Add signature if secret is configured
	if webhook.Secret != "" {
		signature := s.sign(body, webhook.Secret)
		req.Header.Set("X-Webhook-Signature", signature)
	}

	// Send request
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body (limited)
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 10240)) // 10KB max

	return resp.StatusCode, string(respBody), nil
}

// sign generates an HMAC-SHA256 signature.
func (s *Service) sign(payload []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return "sha256=" + hex.EncodeToString(h.Sum(nil))
}

// VerifySignature verifies an incoming webhook signature.
func VerifySignature(payload []byte, signature, secret string) bool {
	expected := "sha256=" + hex.EncodeToString(func() []byte {
		h := hmac.New(sha256.New, []byte(secret))
		h.Write(payload)
		return h.Sum(nil)
	}())
	return hmac.Equal([]byte(signature), []byte(expected))
}

// Stop gracefully stops the webhook service.
func (s *Service) Stop() {
	close(s.stop)
}

// generateID generates a unique delivery ID.
func generateID() string {
	return fmt.Sprintf("wh_%d", time.Now().UnixNano())
}

// GetWebhooks returns all registered webhooks for a tenant.
func (s *Service) GetWebhooks(tenantID int64) []*Webhook {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Webhook
	for _, w := range s.webhooks {
		if w.TenantID == tenantID {
			result = append(result, w)
		}
	}
	return result
}

// GetWebhook returns a webhook by ID.
func (s *Service) GetWebhook(id int64) (*Webhook, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	w, ok := s.webhooks[id]
	if !ok {
		return nil, ErrWebhookNotFound
	}
	return w, nil
}
