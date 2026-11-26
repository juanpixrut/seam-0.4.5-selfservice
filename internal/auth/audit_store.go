package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	xolu "github.com/ha1tch/xolu/pkg/client"
)

// XOLUAuditStore implements AuditStore using XOLU as the backend.
type XOLUAuditStore struct {
	client     *xolu.Client
	entityType string
}

// NewXOLUAuditStore creates a new XOLU-backed audit store.
func NewXOLUAuditStore(client *xolu.Client) *XOLUAuditStore {
	return &XOLUAuditStore{
		client:     client,
		entityType: "audit_logs",
	}
}

// SaveAuditEvent saves an audit event to XOLU.
func (s *XOLUAuditStore) SaveAuditEvent(ctx context.Context, event *AuditEvent) error {
	// Convert metadata to JSON string for storage
	var metadataStr string
	if event.Metadata != nil {
		if data, err := json.Marshal(event.Metadata); err == nil {
			metadataStr = string(data)
		}
	}

	data := map[string]interface{}{
		"event_type": string(event.EventType),
		"user_id":    event.UserID,
		"email":      event.Email,
		"tenant_id":  event.TenantID,
		"ip_address": event.IPAddress,
		"user_agent": event.UserAgent,
		"resource":   event.Resource,
		"action":     event.Action,
		"success":    event.Success,
		"details":    event.Details,
		"metadata":   metadataStr,
		"timestamp":  event.Timestamp.Format(time.RFC3339),
	}

	created, err := s.client.Create(ctx, s.entityType, data)
	if err != nil {
		return err
	}

	event.ID = created.ID
	return nil
}

// ListAuditEvents lists audit events matching the filter.
func (s *XOLUAuditStore) ListAuditEvents(ctx context.Context, filter AuditFilter) ([]*AuditEvent, error) {
	// Set defaults
	limit := filter.Limit
	if limit == 0 {
		limit = 100
	}

	// xolu.Client.List()'s Sort parameter is never sent to the server at
	// all (confirmed directly against pkg/client/client.go: List only
	// ever sets per_page/page query params, never sort) -- no value
	// passed there can ever produce a real ordering. OQL's own ORDER BY
	// does respect real field values for ASC and for the first row of
	// DESC (confirmed by deliberately breaking the id/timestamp
	// correlation and re-querying -- see oql-orderby-diagnostic-
	// report.md), but DESC past the first row falls back to ascending --
	// a real xolu-side bug, reported upstream 2026-08-03. "ORDER BY id
	// DESC" is the one pattern independently confirmed correct
	// end-to-end (id is a real indexed column, not a blob field), and
	// since audit_logs is append-only with auto-incrementing ids, id
	// order and timestamp order coincide in every practical case -- a
	// safe, verified substitute for "ORDER BY timestamp DESC" without
	// waiting on the upstream fix.
	//
	// OQL has no OFFSET/SKIP (confirmed: no such keyword anywhere in
	// xolu's OQL parser), so pagination fetches TOP (limit+offset) and
	// skips the first `offset` of those in Go -- the same "skip N, then
	// take the rest" semantics filter.Offset had against client.List(),
	// just implemented on this side of the fetch instead of the xolu
	// side.
	fetchN := limit + filter.Offset
	query := fmt.Sprintf("SELECT TOP %d * FROM %s ORDER BY id DESC", fetchN, s.entityType)

	result, err := s.client.OQL(ctx, query)
	if err != nil {
		return nil, err
	}

	// Convert to audit events (already newest-first) and apply filters.
	// Fetching more than `limit` (by `offset`) then filtering can still
	// yield fewer than `limit` matches if filter.Resource/EventType(s)
	// exclude many of the fetched rows -- the same limitation the prior
	// client.List()-based version had (it fetched exactly `limit` raw
	// rows before filtering, an even narrower window); not something
	// this fix attempts to solve, since it wasn't what was reported.
	events := make([]*AuditEvent, 0, limit)
	for i, row := range result.Data {
		if i < filter.Offset {
			continue
		}
		entity := xolu.Entity{ID: toInt64(row["id"]), Data: row}
		event := s.entityToEvent(&entity)
		if event == nil {
			continue
		}

		// Apply filters
		if filter.UserID > 0 && event.UserID != filter.UserID {
			continue
		}
		if filter.TenantID > 0 && event.TenantID != filter.TenantID {
			continue
		}
		if filter.EventType != "" && event.EventType != filter.EventType {
			continue
		}
		if filter.Resource != "" && event.Resource != filter.Resource {
			continue
		}
		if len(filter.EventTypes) > 0 {
			matched := false
			for _, et := range filter.EventTypes {
				if event.EventType == et {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		if !filter.StartTime.IsZero() && event.Timestamp.Before(filter.StartTime) {
			continue
		}
		if !filter.EndTime.IsZero() && event.Timestamp.After(filter.EndTime) {
			continue
		}

		events = append(events, event)
		if len(events) >= limit {
			break
		}
	}

	return events, nil
}

// entityToEvent converts an XOLU entity to an AuditEvent.
func (s *XOLUAuditStore) entityToEvent(entity *xolu.Entity) *AuditEvent {
	if entity == nil {
		return nil
	}

	event := &AuditEvent{
		ID:        entity.ID,
		EventType: AuditEventType(toString(entity.Data["event_type"])),
		UserID:    toInt64(entity.Data["user_id"]),
		Email:     toString(entity.Data["email"]),
		TenantID:  toInt64(entity.Data["tenant_id"]),
		IPAddress: toString(entity.Data["ip_address"]),
		UserAgent: toString(entity.Data["user_agent"]),
		Resource:  toString(entity.Data["resource"]),
		Action:    toString(entity.Data["action"]),
		Success:   toBool(entity.Data["success"]),
		Details:   toString(entity.Data["details"]),
	}

	// Parse timestamp
	if ts := toString(entity.Data["timestamp"]); ts != "" {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			event.Timestamp = t
		}
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = entity.CreatedAt
	}

	// Parse metadata
	if metaStr := toString(entity.Data["metadata"]); metaStr != "" {
		var meta map[string]any
		if err := json.Unmarshal([]byte(metaStr), &meta); err == nil {
			event.Metadata = meta
		}
	}

	return event
}

// Helper functions (same as in store_olu.go)

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func toInt64(v interface{}) int64 {
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	}
	return 0
}

func toBool(v interface{}) bool {
	if v == nil {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}
