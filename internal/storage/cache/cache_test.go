package cache

import (
	"context"
	"testing"
	"time"

	"notesmcp/internal/domain"
)

func TestGetObjectCachesValueUntilTTLExpires(t *testing.T) {
	clock := fakeClock{now: time.Unix(100, 0)}
	store := &countingStore{objects: map[string][]byte{"Home.md": []byte("first")}}
	cached := New(store, 30*time.Second, clock.Now)

	first, err := cached.GetObject(context.Background(), "Home.md")
	if err != nil {
		t.Fatalf("first GetObject returned error: %v", err)
	}
	store.objects["Home.md"] = []byte("second")
	second, err := cached.GetObject(context.Background(), "Home.md")
	if err != nil {
		t.Fatalf("second GetObject returned error: %v", err)
	}

	if string(first) != "first" || string(second) != "first" {
		t.Fatalf("cached values = %q and %q, want first and first", first, second)
	}
	if store.getCalls != 1 {
		t.Fatalf("getCalls = %d, want 1", store.getCalls)
	}

	clock.now = clock.now.Add(31 * time.Second)
	third, err := cached.GetObject(context.Background(), "Home.md")
	if err != nil {
		t.Fatalf("third GetObject returned error: %v", err)
	}
	if string(third) != "second" {
		t.Fatalf("third value = %q, want second after TTL", third)
	}
	if store.getCalls != 2 {
		t.Fatalf("getCalls = %d, want 2", store.getCalls)
	}
}

func TestListObjectsCachesDefensiveCopy(t *testing.T) {
	clock := fakeClock{now: time.Unix(100, 0)}
	store := &countingStore{listed: []domain.Object{{Key: "Home.md", Size: 1}}}
	cached := New(store, 30*time.Second, clock.Now)

	first, err := cached.ListObjects(context.Background())
	if err != nil {
		t.Fatalf("first ListObjects returned error: %v", err)
	}
	first[0].Key = "mutated.md"

	second, err := cached.ListObjects(context.Background())
	if err != nil {
		t.Fatalf("second ListObjects returned error: %v", err)
	}
	if second[0].Key != "Home.md" {
		t.Fatalf("cached list was mutated: %#v", second)
	}
	if store.listCalls != 1 {
		t.Fatalf("listCalls = %d, want 1", store.listCalls)
	}
}

func TestPutObjectInvalidatesCachedObjectAndList(t *testing.T) {
	clock := fakeClock{now: time.Unix(100, 0)}
	store := &countingStore{
		objects: map[string][]byte{"Home.md": []byte("first")},
		listed:  []domain.Object{{Key: "Home.md", Size: 5}},
	}
	cached := New(store, 30*time.Second, clock.Now)

	if _, err := cached.GetObject(context.Background(), "Home.md"); err != nil {
		t.Fatalf("warm GetObject returned error: %v", err)
	}
	if _, err := cached.ListObjects(context.Background()); err != nil {
		t.Fatalf("warm ListObjects returned error: %v", err)
	}
	if err := cached.PutObject(context.Background(), "Home.md", []byte("second")); err != nil {
		t.Fatalf("PutObject returned error: %v", err)
	}

	body, err := cached.GetObject(context.Background(), "Home.md")
	if err != nil {
		t.Fatalf("GetObject after PutObject returned error: %v", err)
	}
	if string(body) != "second" {
		t.Fatalf("body = %q, want second", body)
	}
	if store.getCalls != 2 {
		t.Fatalf("getCalls = %d, want 2 after invalidation", store.getCalls)
	}

	objects, err := cached.ListObjects(context.Background())
	if err != nil {
		t.Fatalf("ListObjects after PutObject returned error: %v", err)
	}
	if objects[0].Size != 6 {
		t.Fatalf("object size = %d, want refreshed size 6", objects[0].Size)
	}
}

type fakeClock struct {
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	return c.now
}

type countingStore struct {
	getCalls  int
	listCalls int
	objects   map[string][]byte
	listed    []domain.Object
}

func (s *countingStore) ListObjects(context.Context) ([]domain.Object, error) {
	s.listCalls++
	return append([]domain.Object(nil), s.listed...), nil
}

func (s *countingStore) GetObject(_ context.Context, key string) ([]byte, error) {
	s.getCalls++
	return append([]byte(nil), s.objects[key]...), nil
}

func (s *countingStore) PutObject(_ context.Context, key string, body []byte) error {
	s.objects[key] = append([]byte(nil), body...)
	for i := range s.listed {
		if s.listed[i].Key == key {
			s.listed[i].Size = int64(len(body))
			return nil
		}
	}
	s.listed = append(s.listed, domain.Object{Key: key, Size: int64(len(body))})
	return nil
}
