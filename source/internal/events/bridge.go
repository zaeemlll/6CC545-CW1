package events

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	forwardSendTimeout = 2 * time.Second
	forwardMaxAttempts = 2
)

// Forward creates a subscription and forwards events from its topic.
func (b *Bus) Forward(
	ctx context.Context,
	topic string,
	send func(context.Context, Event) error,
) error {
	if ctx == nil {
		return errors.New("forward context is nil")
	}
	if send == nil {
		return errors.New("forward send callback is nil")
	}

	sub, err := b.Subscribe(topic)
	if err != nil {
		return err
	}

	return b.ForwardSubscription(ctx, sub, send)
}

// ForwardSubscription forwards events from an existing subscription.
func (b *Bus) ForwardSubscription(
	ctx context.Context,
	sub *Subscription,
	send func(context.Context, Event) error,
) error {
	if ctx == nil {
		return errors.New("forward context is nil")
	}
	if sub == nil {
		return errors.New("forward subscription is nil")
	}
	if send == nil {
		return errors.New("forward send callback is nil")
	}

	defer sub.Cancel()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case event, ok := <-sub.C:
			if !ok {
				return nil
			}

			var sendErr error

			for attempt := 1; attempt <= forwardMaxAttempts; attempt++ {
				if err := ctx.Err(); err != nil {
					return err
				}

				sendCtx, cancel := context.WithTimeout(ctx, forwardSendTimeout)
				sendErr = send(sendCtx, event)
				cancel()

				if sendErr == nil {
					break
				}

				if ctx.Err() != nil {
					return ctx.Err()
				}
			}

			if sendErr != nil {
				return fmt.Errorf(
					"forward event %q failed after %d attempts: %w",
					event.ID,
					forwardMaxAttempts,
					sendErr,
				)
			}
		}
	}
}
