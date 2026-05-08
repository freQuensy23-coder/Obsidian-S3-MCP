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

func TestReplaceInNoteChangesRequestedFragmentAndPersists(t *testing.T) {
	store := fakeStore{
		{key: "Home.md", size: 16, body: []byte("alpha beta tail")},
	}
	service := NewService(store)

	note, err := service.ReplaceInNote(context.Background(), "Home.md", "beta", "gamma", false)
	if err != nil {
		t.Fatalf("ReplaceInNote returned error: %v", err)
	}

	if note.Body != "alpha gamma tail" {
		t.Fatalf("Body = %q, want one replacement", note.Body)
	}
	if string(store.body("Home.md")) != "alpha gamma tail" {
		t.Fatalf("persisted body = %q", store.body("Home.md"))
	}
}

func TestReplaceInNoteRejectsAmbiguousFragmentUnlessReplaceAll(t *testing.T) {
	store := fakeStore{
		{key: "Home.md", size: 14, body: []byte("beta beta beta")},
	}
	service := NewService(store)

	_, err := service.ReplaceInNote(context.Background(), "Home.md", "beta", "gamma", false)
	if err == nil {
		t.Fatal("ReplaceInNote succeeded with ambiguous old_text")
	}
	if string(store.body("Home.md")) != "beta beta beta" {
		t.Fatalf("body changed after rejected edit: %q", store.body("Home.md"))
	}

	note, err := service.ReplaceInNote(context.Background(), "Home.md", "beta", "gamma", true)
	if err != nil {
		t.Fatalf("ReplaceInNote replace_all returned error: %v", err)
	}
	if note.Body != "gamma gamma gamma" {
		t.Fatalf("Body = %q, want all replacements", note.Body)
	}
}

func TestAddNoteTagsAddsObsidianTagLinksWithoutDuplicates(t *testing.T) {
	store := fakeStore{
		{key: "0000.Work.md", size: 0, body: []byte("")},
		{key: "0000.Life.md", size: 0, body: []byte("")},
		{key: "0001.ML.md", size: 0, body: []byte("")},
		{key: "Home.md", size: 21, body: []byte("[[0000.Work]]\nbody")},
	}
	service := NewService(store)

	note, err := service.AddNoteTags(context.Background(), "Home.md", []string{"Work", "life", "[[ML]]"})
	if err != nil {
		t.Fatalf("AddNoteTags returned error: %v", err)
	}

	want := "[[0000.Work]] [[0000.Life]] [[0001.ML]]\nbody"
	if note.Body != want {
		t.Fatalf("Body = %q, want %q", note.Body, want)
	}
}

func TestAddNoteTagsRejectsUnknownTag(t *testing.T) {
	store := fakeStore{
		{key: "0000.Work.md", size: 0, body: []byte("")},
		{key: "Home.md", size: 4, body: []byte("body")},
	}
	service := NewService(store)

	_, err := service.AddNoteTags(context.Background(), "Home.md", []string{"Unknown"})
	if err == nil {
		t.Fatal("AddNoteTags succeeded with unknown tag")
	}
	if string(store.body("Home.md")) != "body" {
		t.Fatalf("body changed after rejected tag: %q", store.body("Home.md"))
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

func (s fakeStore) PutObject(_ context.Context, key string, body []byte) error {
	for i := range s {
		if s[i].key == key {
			s[i].body = append([]byte(nil), body...)
			s[i].size = int64(len(body))
			return nil
		}
	}
	s = append(s, fakeObject{key: key, size: int64(len(body)), body: append([]byte(nil), body...)})
	return nil
}

func (s fakeStore) body(key string) []byte {
	for _, object := range s {
		if object.key == key {
			return object.body
		}
	}
	return nil
}
