package vault

import (
	"context"
	"testing"
	"time"

	"notesmcp/internal/domain"
)

func TestOverviewCountsVaultObjects(t *testing.T) {
	service := NewService(fakeStore{
		{key: "Home.md", size: 33},
		{key: "Tasks/Task.md", size: 100},
		{key: "Materials/image.png", size: 200},
	})

	overview, err := service.Overview(context.Background())
	if err != nil {
		t.Fatalf("Overview returned error: %v", err)
	}

	if overview.TotalObjects != 3 {
		t.Fatalf("TotalObjects = %d, want 3", overview.TotalObjects)
	}
	if overview.MarkdownNotes != 2 {
		t.Fatalf("MarkdownNotes = %d, want 2", overview.MarkdownNotes)
	}
	if overview.Folders["Tasks"] != 1 {
		t.Fatalf("Tasks count = %d, want 1", overview.Folders["Tasks"])
	}
}

func TestBacklinksUsesObsidianWikilinks(t *testing.T) {
	service := NewService(fakeStore{
		{key: "Haskell.md", body: []byte("target")},
		{key: "Dataview.md", body: []byte("[[Haskell]]")},
		{key: "Other.md", body: []byte("[[Missing]]")},
	})

	links, err := service.Backlinks(context.Background(), "Haskell.md")
	if err != nil {
		t.Fatalf("Backlinks returned error: %v", err)
	}

	if len(links) != 1 || links[0].Key != "Dataview.md" {
		t.Fatalf("Backlinks = %#v, want Dataview.md", links)
	}
}

type fakeStore []fakeObject

type fakeObject struct {
	key  string
	size int64
	body []byte
}

func (s fakeStore) ListObjects(context.Context) ([]domain.Object, error) {
	out := make([]domain.Object, 0, len(s))
	for _, object := range s {
		out = append(out, domain.Object{Key: object.key, Size: object.size, LastModified: time.Unix(0, 0)})
	}
	return out, nil
}

func (s fakeStore) GetObject(_ context.Context, key string) ([]byte, error) {
	for _, object := range s {
		if object.key == key {
			return object.body, nil
		}
	}
	return nil, domain.ErrNotFound
}
