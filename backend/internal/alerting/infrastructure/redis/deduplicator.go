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

func (d *Deduplicator) ShouldDispatch(ctx context.Context, alert sharedDomain.AlertEvent) (bool, error) {
	key := d.prefix + alert.Fingerprint

	created, err := d.client.SetNX(
		ctx,
		key,
		alert.EventID,
		d.period,
	).Result()
	if err != nil {
		return false, fmt.Errorf("set dedup key %q: %w", key, err)
	}

	return created, nil
}
