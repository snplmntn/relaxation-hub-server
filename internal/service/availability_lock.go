package service

import (
	"context"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
)

type availabilityLockWindow struct {
	Start time.Time
	End   time.Time
}

func lockAvailabilityWindows(ctx context.Context, tx pgx.Tx, windows []availabilityLockWindow) error {
	if tx == nil || len(windows) == 0 {
		return nil
	}

	keys := make(map[int64]struct{})
	for _, window := range windows {
		if window.Start.IsZero() || window.End.IsZero() || !window.Start.Before(window.End) {
			continue
		}
		for bucket := availabilityLockBucket(window.Start); bucket <= availabilityLockBucket(window.End.Add(-time.Second)); bucket++ {
			keys[bucket] = struct{}{}
		}
	}

	ordered := make([]int64, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })

	for _, key := range ordered {
		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", key); err != nil {
			return err
		}
	}
	return nil
}

func availabilityLockBucket(t time.Time) int64 {
	const lockNamespace int64 = 424200000000
	return lockNamespace + t.UTC().Unix()/1800
}
