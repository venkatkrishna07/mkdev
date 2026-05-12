package store

import (
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

// SourceAdHoc marks routes added directly via the CLI (not from a project file).
const SourceAdHoc = "ad-hoc"

// Route describes a domain to upstream mapping.
type Route struct {
	Domain  string    `json:"domain"`
	Target  string    `json:"target"` // host:port
	TLD     string    `json:"tld"`
	Enabled bool      `json:"enabled"`
	Source  string    `json:"source"` // "ad-hoc" or absolute project path
	AddedAt time.Time `json:"added_at"`
}

// PutRoute inserts or replaces a route keyed by Domain.
func (s *Store) PutRoute(r Route) error {
	if r.Domain == "" {
		return fmt.Errorf("store: route domain empty")
	}
	buf, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("store: marshal: %w", err)
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketRoutes).Put([]byte(r.Domain), buf)
	})
}

// GetRoute returns the route for the given domain.
func (s *Store) GetRoute(domain string) (Route, error) {
	var r Route
	err := s.db.View(func(tx *bolt.Tx) error {
		buf := tx.Bucket(bucketRoutes).Get([]byte(domain))
		if buf == nil {
			return ErrNotFound
		}
		return json.Unmarshal(buf, &r)
	})
	return r, err
}

// DeleteRoute removes a route. Missing keys are not an error.
func (s *Store) DeleteRoute(domain string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketRoutes).Delete([]byte(domain))
	})
}

// ListRoutes returns all routes in lexicographic order by domain.
func (s *Store) ListRoutes() ([]Route, error) {
	out := []Route{}
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketRoutes).ForEach(func(_, v []byte) error {
			var r Route
			if err := json.Unmarshal(v, &r); err != nil {
				return err
			}
			out = append(out, r)
			return nil
		})
	})
	return out, err
}
