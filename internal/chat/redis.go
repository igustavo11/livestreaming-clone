package chat

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// RedisPubSub implements PubSub via Redis pub/sub.
type RedisPubSub struct {
	client *redis.Client
}

func NewRedisPubSub(addr string) (*RedisPubSub, error) {
	c := redis.NewClient(&redis.Options{Addr: addr})
	if err := c.Ping(context.Background()).Err(); err != nil {
		_ = c.Close()
		return nil, err
	}
	return &RedisPubSub{client: c}, nil
}

func (r *RedisPubSub) Publish(ctx context.Context, channel, message string) error {
	return r.client.Publish(ctx, channel, message).Err()
}

func (r *RedisPubSub) Subscribe(ctx context.Context, channel string) (<-chan string, func(), error) {
	pubsub := r.client.Subscribe(ctx, channel)
	// wait for subscription confirmation
	if _, err := pubsub.Receive(ctx); err != nil {
		_ = pubsub.Close()
		return nil, nil, err
	}
	ch := make(chan string, 64)
	go func() {
		defer close(ch)
		c := pubsub.Channel()
		for msg := range c {
			select {
			case ch <- msg.Payload:
			case <-ctx.Done():
				return
			}
		}
	}()
	unsub := func() {
		_ = pubsub.Close()
	}
	return ch, unsub, nil
}

func (r *RedisPubSub) Close() error {
	return r.client.Close()
}
