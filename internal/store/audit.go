package store

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/sunpe/doraemon/internal/audit"
	"go.etcd.io/bbolt"
)

var bucketAudit = []byte("audit")

func (d *DB) WriteAudit(event audit.Event) error {
	if event.ID == "" {
		event.ID = "evt_" + randomString(18)
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	return d.db.Update(func(tx *bbolt.Tx) error {
		return putJSON(tx.Bucket(bucketAudit), event.Timestamp.Format(time.RFC3339Nano)+"_"+event.ID, event)
	})
}

func (d *DB) ListAudit(since time.Time, limit int) ([]audit.Event, error) {
	if limit <= 0 {
		limit = 100
	}
	var events []audit.Event
	err := d.db.View(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketAudit).ForEach(func(_, v []byte) error {
			var event audit.Event
			if err := json.Unmarshal(v, &event); err != nil {
				return err
			}
			if !since.IsZero() && event.Timestamp.Before(since) {
				return nil
			}
			if len(events) >= limit {
				return fmt.Errorf("audit limit reached")
			}
			events = append(events, event)
			return nil
		})
	})
	if err != nil && err.Error() == "audit limit reached" {
		err = nil
	}
	return events, err
}
