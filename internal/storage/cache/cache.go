package cache

import (
	"context"
	"sync"
	"time"

	"notesmcp/internal/domain"
)

type Store struct {
	next domain.ObjectStore
	ttl  time.Duration
	now  func() time.Time

	mu        sync.Mutex
	objects   map[string]objectEntry
	list      []domain.Object
	listUntil time.Time
}

type objectEntry struct {
	body      []byte
	expiresAt time.Time
}

func New(next domain.ObjectStore, ttl time.Duration, now func() time.Time) *Store {
	if now == nil {
		now = time.Now
	}
	return &Store{
		next:    next,
		ttl:     ttl,
		now:     now,
		objects: map[string]objectEntry{},
	}
}

func (s *Store) ListObjects(ctx context.Context) ([]domain.Object, error) {
	s.mu.Lock()
	now := s.now()
	if s.list != nil && now.Before(s.listUntil) {
		out := cloneObjects(s.list)
		s.mu.Unlock()
		return out, nil
	}
	s.mu.Unlock()

	objects, err := s.next.ListObjects(ctx)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.list = cloneObjects(objects)
	s.listUntil = now.Add(s.ttl)
	out := cloneObjects(s.list)
	s.mu.Unlock()
	return out, nil
}

func (s *Store) GetObject(ctx context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	now := s.now()
	if entry, ok := s.objects[key]; ok && now.Before(entry.expiresAt) {
		body := cloneBytes(entry.body)
		s.mu.Unlock()
		return body, nil
	}
	s.mu.Unlock()

	body, err := s.next.GetObject(ctx, key)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.objects[key] = objectEntry{body: cloneBytes(body), expiresAt: now.Add(s.ttl)}
	out := cloneBytes(body)
	s.mu.Unlock()
	return out, nil
}

func (s *Store) PutObject(ctx context.Context, key string, body []byte) error {
	if err := s.next.PutObject(ctx, key, body); err != nil {
		return err
	}

	s.mu.Lock()
	delete(s.objects, key)
	s.list = nil
	s.listUntil = time.Time{}
	s.mu.Unlock()
	return nil
}

func cloneObjects(objects []domain.Object) []domain.Object {
	return append([]domain.Object(nil), objects...)
}

func cloneBytes(body []byte) []byte {
	return append([]byte(nil), body...)
}
