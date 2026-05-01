package main

import (
	"sync"
	"time"
)

const (
	defaultDeliveryRetention    = 24 * time.Hour
	defaultDeliveryCacheEntries = 10000
)

type deliveryDeduper struct {
	mu         sync.Mutex
	ttl        time.Duration
	maxEntries int
	entries    map[string]time.Time
}

func newDeliveryDeduper(ttl time.Duration, maxEntries int) *deliveryDeduper {
	return &deliveryDeduper{
		ttl:        ttl,
		maxEntries: maxEntries,
		entries:    make(map[string]time.Time),
	}
}

func (d *deliveryDeduper) SeenBefore(deliveryID string, now time.Time) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.pruneExpired(now)

	if expiresAt, ok := d.entries[deliveryID]; ok {
		if now.Before(expiresAt) {
			return true
		}
		delete(d.entries, deliveryID)
	}

	d.entries[deliveryID] = now.Add(d.ttl)
	d.evictOverflow()

	return false
}

func (d *deliveryDeduper) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.entries = make(map[string]time.Time)
}

func (d *deliveryDeduper) pruneExpired(now time.Time) {
	for deliveryID, expiresAt := range d.entries {
		if !now.Before(expiresAt) {
			delete(d.entries, deliveryID)
		}
	}
}

func (d *deliveryDeduper) evictOverflow() {
	for len(d.entries) > d.maxEntries {
		var (
			oldestID string
			oldestAt time.Time
		)

		for deliveryID, expiresAt := range d.entries {
			if oldestID == "" || expiresAt.Before(oldestAt) {
				oldestID = deliveryID
				oldestAt = expiresAt
			}
		}

		delete(d.entries, oldestID)
	}
}
