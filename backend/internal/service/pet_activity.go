package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const petActivityQueueSize = 2048

type PetActivityEvent struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	Status     string    `json:"status"`
	Endpoint   string    `json:"endpoint,omitempty"`
	RequestID  string    `json:"request_id,omitempty"`
	OccurredAt time.Time `json:"occurred_at"`
}

type queuedPetActivity struct {
	userID int64
	event  PetActivityEvent
}

type PetActivityBroker struct {
	redis *redis.Client
	queue chan queuedPetActivity
}

func NewPetActivityBroker(redisClient *redis.Client) *PetActivityBroker {
	b := &PetActivityBroker{redis: redisClient, queue: make(chan queuedPetActivity, petActivityQueueSize)}
	go b.publishLoop()
	return b
}

func (b *PetActivityBroker) Emit(userID int64, event PetActivityEvent) {
	if b == nil || b.redis == nil || userID <= 0 {
		return
	}
	if event.ID == "" {
		event.ID = uuid.NewString()
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	select {
	case b.queue <- queuedPetActivity{userID: userID, event: event}:
	default:
	}
}

func (b *PetActivityBroker) publishLoop() {
	for item := range b.queue {
		body, err := json.Marshal(item.event)
		if err != nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = b.redis.Publish(ctx, petActivityChannel(item.userID), body).Err()
		cancel()
	}
}

func (b *PetActivityBroker) Subscribe(ctx context.Context, userID int64) (<-chan PetActivityEvent, func(), error) {
	if b == nil || b.redis == nil {
		return nil, nil, fmt.Errorf("pet activity broker unavailable")
	}
	pubsub := b.redis.Subscribe(ctx, petActivityChannel(userID))
	if _, err := pubsub.Receive(ctx); err != nil {
		_ = pubsub.Close()
		return nil, nil, err
	}
	out := make(chan PetActivityEvent, 32)
	cancelCtx, cancel := context.WithCancel(ctx)
	go func() {
		defer close(out)
		for {
			select {
			case <-cancelCtx.Done():
				return
			case message, ok := <-pubsub.Channel():
				if !ok {
					return
				}
				var event PetActivityEvent
				if json.Unmarshal([]byte(message.Payload), &event) == nil {
					select {
					case out <- event:
					case <-cancelCtx.Done():
						return
					}
				}
			}
		}
	}()
	cleanup := func() { cancel(); _ = pubsub.Close() }
	return out, cleanup, nil
}

func petActivityChannel(userID int64) string { return fmt.Sprintf("pet:activity:user:%d", userID) }
