package vault

import (
	"context"
	"errors"
	"path/filepath"
	"regexp"
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

func (s *Service) WriteNote(ctx context.Context, key, body string) (domain.Note, error) {
	if !isMarkdown(key) {
		return domain.Note{}, errors.New("only markdown notes can be written")
	}
	if err := s.store.PutObject(ctx, key, []byte(body)); err != nil {
		return domain.Note{}, err
	}
	return s.GetNote(ctx, key)
}

func (s *Service) AppendNote(ctx context.Context, key, text string) (domain.Note, error) {
	note, err := s.GetNote(ctx, key)
	if err != nil {
		return domain.Note{}, err
	}
	body := note.Body
	if body != "" && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	body += text
	return s.WriteNote(ctx, key, body)
}

func (s *Service) ReplaceInNote(ctx context.Context, key, oldText, newText string, replaceAll bool) (domain.Note, error) {
	if oldText == "" {
		return domain.Note{}, errors.New("old_text is required")
	}
	note, err := s.GetNote(ctx, key)
	if err != nil {
		return domain.Note{}, err
	}
	if !strings.Contains(note.Body, oldText) {
		return domain.Note{}, errors.New("old_text not found")
	}
	if !replaceAll && strings.Count(note.Body, oldText) != 1 {
		return domain.Note{}, errors.New("old_text is not unique; provide a larger exact fragment or set replace_all=true")
	}

	count := 1
	if replaceAll {
		count = -1
	}
	body := strings.Replace(note.Body, oldText, newText, count)
	return s.WriteNote(ctx, key, body)
}

func (s *Service) AddNoteTags(ctx context.Context, key string, tags []string) (domain.Note, error) {
	note, err := s.GetNote(ctx, key)
	if err != nil {
		return domain.Note{}, err
	}

	links, err := s.resolveTagLinks(ctx, tags)
	if err != nil {
		return domain.Note{}, err
	}
	if len(links) == 0 {
		return note, nil
	}

	existing := map[string]struct{}{}
	for _, link := range links {
		if strings.Contains(note.Body, link) {
			existing[link] = struct{}{}
		}
	}

	missing := make([]string, 0, len(links))
	for _, link := range links {
		if _, ok := existing[link]; !ok {
			missing = append(missing, link)
		}
	}
	if len(missing) == 0 {
		return note, nil
	}

	body := note.Body
	firstLine, rest, hasRest := strings.Cut(body, "\n")
	if strings.HasPrefix(strings.TrimSpace(firstLine), "[[") {
		firstLine = strings.TrimSpace(firstLine) + " " + strings.Join(missing, " ")
		if hasRest {
			body = firstLine + "\n" + rest
		} else {
			body = firstLine
		}
	} else {
		prefix := strings.Join(missing, " ")
		if strings.TrimSpace(body) == "" {
			body = prefix
		} else {
			body = prefix + "\n" + body
		}
	}

	return s.WriteNote(ctx, key, body)
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

var tagNoteRE = regexp.MustCompile(`^\d{4}\..+\.md$`)

func (s *Service) resolveTagLinks(ctx context.Context, tags []string) ([]string, error) {
	notes, err := s.ListNotes(ctx)
	if err != nil {
		return nil, err
	}

	candidates := map[string]string{}
	for _, note := range notes {
		base := filepath.Base(note.Key)
		if !tagNoteRE.MatchString(base) {
			continue
		}
		title := strings.TrimSuffix(base, ".md")
		candidates[strings.ToLower(title)] = title
		parts := strings.SplitN(title, ".", 2)
		if len(parts) == 2 {
			candidates[strings.ToLower(parts[1])] = title
		}
	}

	seen := map[string]struct{}{}
	links := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		tag = strings.TrimPrefix(tag, "[[")
		tag = strings.TrimSuffix(tag, "]]")
		tag = strings.TrimSuffix(tag, ".md")
		if tag == "" {
			continue
		}
		resolved, ok := candidates[strings.ToLower(tag)]
		if !ok {
			return nil, errors.New("tag note not found: " + tag)
		}
		link := "[[" + resolved + "]]"
		if _, ok := seen[link]; ok {
			continue
		}
		seen[link] = struct{}{}
		links = append(links, link)
	}
	return links, nil
}
