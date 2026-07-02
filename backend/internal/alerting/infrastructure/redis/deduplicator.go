package redis

import (
	"context"
	"fmt"
	"time"

	sharedDomain "github.com/n0thing2c/Soigineer/internal/shared/domain"
	goredis "github.com/redis/go-redis/v9"
)

type Deduplicator struct {
	client *goredis.Client
	period time.Duration
	prefix string
}

func NewDeduplicator(client *goredis.Client, period time.Duration, prefix string) *Deduplicator {
	if period <= 0 {
		period = time.Minute
	}
	return &Deduplicator{
		client: client,
		period: period,
		prefix: prefix,
	}
}

func (d *Deduplicator) ShouldDispatch(
	ctx context.Context,
	alert sharedDomain.AlertEvent,
	window time.Duration,
) (bool, error) {
	key := d.prefix + alert.Fingerprint
	period := d.period
	if window > 0 {
		period = window
	}

	created, err := d.client.SetNX(
		ctx,
		key,
		alert.EventID,
		period,
	).Result()
	if err != nil {
		return false, fmt.Errorf("set dedup key %q: %w", key, err)
	}

	return created, nil
}
