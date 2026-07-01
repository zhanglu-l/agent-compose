package agentcompose

import (
	appconfig "agent-compose/pkg/config"

	"agent-compose/pkg/agentcompose/webhooks"
)

type (
	WebhookRunQueue         = webhooks.RunQueue
	webhookQueueReservation = webhooks.Reservation
)

func noopWebhookQueueReservations(count int) []*webhookQueueReservation {
	return webhooks.NoopReservations(count)
}

func newWebhookRunQueueFromConfig(config *appconfig.Config) (*WebhookRunQueue, error) {
	return webhooks.NewRunQueueFromConfig(config)
}

func newWebhookRunQueue(defaultWorkers int) *WebhookRunQueue {
	return webhooks.NewRunQueue(defaultWorkers)
}
