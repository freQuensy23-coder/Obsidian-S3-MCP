package vault

import (
	"context"
	"path/filepath"
	"sort"
	"strings"

	"notesmcp/internal/domain"
	"notesmcp/internal/obsidian"
)

type Service struct {
	store domain.ObjectStore
}

func NewService(store domain.ObjectStore) *Service {
	return &Service{store: store}
}

func (s *Service) Overview(ctx context.Context) (domain.Overview, error) {
	objects, err := s.store.ListObjects(ctx)
	if err != nil {
		return domain.Overview{}, err
	}

	overview := domain.Overview{
		Folders:    map[string]int{},
		Extensions: map[string]int{},
	}
	for _, object := range objects {
		overview.TotalObjects++
		overview.TotalBytes += object.Size
		if isMarkdown(object.Key) {
			overview.MarkdownNotes++
		}
		overview.Folders[topFolder(object.Key)]++
		overview.Extensions[extension(object.Key)]++
	}
	return overview, nil
}

func (s *Service) ListNotes(ctx context.Context) ([]domain.Note, error) {
	objects, err := s.store.ListObjects(ctx)
	if err != nil {
		return nil, err
	}

	notes := make([]domain.Note, 0)
	for _, object := range objects {
		if !isMarkdown(object.Key) {
			continue
		}
		notes = append(notes, domain.Note{Object: object, Title: titleFromKey(object.Key)})
	}
	sort.Slice(notes, func(i, j int) bool { return notes[i].Key < notes[j].Key })
	return notes, nil
}

func (s *Service) GetNote(ctx context.Context, key string) (domain.Note, error) {
	objects, err := s.store.ListObjects(ctx)
	if err != nil {
		return domain.Note{}, err
	}
	var meta domain.Object
	found := false
	for _, object := range objects {
		if object.Key == key {
			meta = object
			found = true
			break
		}
	}
	if !found || !isMarkdown(key) {
		return domain.Note{}, domain.ErrNotFound
	}

	body, err := s.store.GetObject(ctx, key)
	if err != nil {
		return domain.Note{}, err
	}

	return domain.Note{Object: meta, Title: titleFromKey(key), Body: string(body)}, nil
}

func (s *Service) SearchNotes(ctx context.Context, query string, limit int) ([]domain.Note, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	query = strings.ToLower(strings.TrimSpace(query))
	notes, err := s.ListNotes(ctx)
	if err != nil {
		return nil, err
	}
	if query == "" {
		if len(notes) > limit {
			return notes[:limit], nil
		}
		return notes, nil
	}

	results := make([]domain.Note, 0)
	for _, note := range notes {
		if strings.Contains(strings.ToLower(note.Key), query) || strings.Contains(strings.ToLower(note.Title), query) {
			results = append(results, note)
		} else {
			body, err := s.store.GetObject(ctx, note.Key)
			if err != nil {
				return nil, err
			}
			if strings.Contains(strings.ToLower(string(body)), query) {
				results = append(results, note)
			}
		}
		if len(results) >= limit {
			break
		}
	}
	return results, nil
}

func (s *Service) Backlinks(ctx context.Context, key string) ([]domain.Note, error) {
	notes, err := s.ListNotes(ctx)
	if err != nil {
		return nil, err
	}

	metas := make([]obsidian.NoteMeta, 0, len(notes))
	for _, note := range notes {
		metas = append(metas, obsidian.NoteMeta{Key: note.Key, Title: note.Title})
	}
	index := obsidian.NewIndex(metas)

	results := make([]domain.Note, 0)
	for _, note := range notes {
		body, err := s.store.GetObject(ctx, note.Key)
		if err != nil {
			return nil, err
		}
		parsed := obsidian.ParseMarkdown(note.Key, string(body))
		for _, outlink := range parsed.Outlinks {
			resolved, ok := index.ResolveWikilink(outlink)
			if ok && resolved == key {
				results = append(results, note)
				break
			}
		}
	}
	return results, nil
}

func isMarkdown(key string) bool {
	return strings.EqualFold(filepath.Ext(key), ".md")
}

func topFolder(key string) string {
	parts := strings.SplitN(key, "/", 2)
	if len(parts) == 1 {
		return "<root>"
	}
	return parts[0]
}

func extension(key string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(key)), ".")
	if ext == "" {
		return "<none>"
	}
	return ext
}

func titleFromKey(key string) string {
	base := filepath.Base(key)
	return strings.TrimSuffix(base, filepath.Ext(base))
}
