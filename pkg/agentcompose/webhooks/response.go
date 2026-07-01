package webhooks

import (
	"encoding/json"
	"strings"
	"time"

	"agent-compose/pkg/agentcompose/domain"
)

func SourceToJSON(source domain.WebhookSource) SourceJSON {
	return SourceJSON{
		ID:                 source.ID,
		Name:               source.Name,
		Enabled:            source.Enabled,
		Provider:           source.Provider,
		TopicPrefix:        source.TopicPrefix,
		HasToken:           strings.TrimSpace(source.TokenHash) != "",
		SignatureType:      source.SignatureType,
		HasSignatureSecret: strings.TrimSpace(source.SignatureSecret) != "",
		BodyLimitBytes:     source.BodyLimitBytes,
		CreatedAt:          source.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:          source.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func TopicEventToJSON(item domain.TopicEventRecord) TopicEventJSON {
	payload := make(map[string]any)
	_ = json.Unmarshal([]byte(item.PayloadJSON), &payload)
	out := TopicEventJSON{
		EventID:        item.ID,
		Sequence:       item.Sequence,
		Topic:          item.Topic,
		Source:         item.Source,
		Provider:       item.Provider,
		Intent:         item.Intent,
		CorrelationID:  item.CorrelationID,
		IdempotencyKey: item.IdempotencyKey,
		DeliveryID:     item.DeliveryID,
		DispatchStatus: item.DispatchStatus,
		ParentEventID:  item.ParentEventID,
		PublisherType:  item.PublisherType,
		PublisherID:    item.PublisherID,
		PublisherRunID: item.PublisherRunID,
		CreatedAt:      item.CreatedAt.UTC().Format(time.RFC3339Nano),
		Payload:        payload,
	}
	if !item.DispatchedAt.IsZero() {
		out.DispatchedAt = item.DispatchedAt.UTC().Format(time.RFC3339Nano)
	}
	return out
}
