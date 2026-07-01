package events

import (
	"database/sql"
	"fmt"

	"agent-compose/pkg/agentcompose/configstore"
	"agent-compose/pkg/agentcompose/domain"
)

func ScanTopicEvents(rows *sql.Rows) ([]domain.TopicEventRecord, error) {
	items := make([]domain.TopicEventRecord, 0)
	for rows.Next() {
		item, err := ScanTopicEvent(rows.Scan)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}
	return items, nil
}

func ScanTopicEvent(scan func(dest ...any) error) (domain.TopicEventRecord, error) {
	var item domain.TopicEventRecord
	var claimUntilRaw int64
	var nextAttemptAtRaw int64
	var deadLetterAtRaw int64
	var createdAtRaw int64
	var dispatchedAtRaw int64
	if err := scan(
		&item.Sequence,
		&item.ID,
		&item.Topic,
		&item.Source,
		&item.Provider,
		&item.Intent,
		&item.CorrelationID,
		&item.IdempotencyKey,
		&item.DeliveryID,
		&item.PayloadHash,
		&item.PayloadJSON,
		&item.DispatchStatus,
		&item.ParentEventID,
		&item.PublisherType,
		&item.PublisherID,
		&item.PublisherRunID,
		&item.ReplayOfEventID,
		&item.ClaimID,
		&claimUntilRaw,
		&item.AttemptCount,
		&nextAttemptAtRaw,
		&item.LastError,
		&deadLetterAtRaw,
		&createdAtRaw,
		&dispatchedAtRaw,
	); err != nil {
		return domain.TopicEventRecord{}, fmt.Errorf("scan event: %w", err)
	}
	item.ClaimUntil = configstore.ParseStoredUnixTimeAuto(claimUntilRaw)
	item.NextAttemptAt = configstore.ParseStoredUnixTimeAuto(nextAttemptAtRaw)
	item.DeadLetterAt = configstore.ParseStoredUnixTimeAuto(deadLetterAtRaw)
	item.CreatedAt = configstore.ParseStoredUnixTimeAuto(createdAtRaw)
	item.DispatchedAt = configstore.ParseStoredUnixTimeAuto(dispatchedAtRaw)
	return item, nil
}
