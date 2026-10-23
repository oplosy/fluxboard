package chat

import "context"

// Repository persists channels and their deliveries ([T], RLS).
type Repository interface {
	Create(ctx context.Context, orgID string, c *Channel) error
	List(ctx context.Context, orgID string) ([]Channel, error)
	// Get returns domain.ErrNotFound when the channel is not in the org.
	Get(ctx context.Context, orgID, id string) (*Channel, error)
	// Update rewrites name, categories and enabled; ErrNotFound if absent.
	Update(ctx context.Context, orgID string, c *Channel) error
	// Delete removes the channel and its deliveries; ErrNotFound if absent.
	Delete(ctx context.Context, orgID, id string) error

	// Enqueue inserts the delivery and its chat:send outbox row in ONE tx.
	Enqueue(ctx context.Context, orgID string, d *Delivery) error
	// ListDeliveries returns the newest deliveries (channel name joined).
	ListDeliveries(ctx context.Context, orgID string, limit int) ([]Delivery, error)
	// GetDelivery returns domain.ErrNotFound when absent.
	GetDelivery(ctx context.Context, orgID, id string) (*Delivery, error)
	// RecordAttempt bumps attempts and stores the outcome; status sent also
	// stamps sent_at.
	RecordAttempt(ctx context.Context, orgID, id, status, lastError string) error

	// TaskPath resolves the org slug, project key and number of a live task
	// for the deep link; ErrNotFound if the task is gone.
	TaskPath(ctx context.Context, orgID, taskID string) (slug, projectKey string, number int, err error)
}
