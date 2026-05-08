package obsidian

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type NoteMeta struct {
	Key   string `json:"key"`
	Title string `json:"title"`
}

type ParsedNote struct {
	Key      string   `json:"key"`
	Title    string   `json:"title"`
	Tags     []string `json:"tags"`
	Outlinks []string `json:"outlinks"`
	Embeds   []string `json:"embeds"`
}

type Index struct {
	byTitle map[string]string
	byKey   map[string]string
}

var (
	wikilinkRE = regexp.MustCompile(`!?\[\[([^\]|#]+)(?:#[^\]|]+)?(?:\|[^\]]+)?\]\]`)
	tagRE      = regexp.MustCompile(`(?:^|\s)#([[:alnum:]_-]+)`)
)

func ParseMarkdown(key, body string) ParsedNote {
	seenLinks := map[string]struct{}{}
	seenEmbeds := map[string]struct{}{}
	seenTags := map[string]struct{}{}

	for _, match := range wikilinkRE.FindAllStringSubmatchIndex(body, -1) {
		fullStart := match[0]
		targetStart := match[2]
		targetEnd := match[3]
		target := strings.TrimSpace(body[targetStart:targetEnd])
		if target == "" {
			continue
		}
		if body[fullStart] == '!' {
			seenEmbeds[target] = struct{}{}
			continue
		}
		seenLinks[target] = struct{}{}
	}

	for _, match := range tagRE.FindAllStringSubmatch(body, -1) {
		seenTags[match[1]] = struct{}{}
	}

	return ParsedNote{
		Key:      key,
		Title:    titleFromKey(key),
		Tags:     sortedKeys(seenTags),
		Outlinks: sortedKeys(seenLinks),
		Embeds:   sortedKeys(seenEmbeds),
	}
}

func NewIndex(notes []NoteMeta) Index {
	index := Index{byTitle: map[string]string{}, byKey: map[string]string{}}
	for _, note := range notes {
		index.byKey[note.Key] = note.Key
		index.byTitle[note.Title] = note.Key
		index.byTitle[titleFromKey(note.Key)] = note.Key
	}
	return index
}

func (i Index) ResolveWikilink(link string) (string, bool) {
	target := strings.TrimSpace(strings.SplitN(strings.SplitN(link, "|", 2)[0], "#", 2)[0])
	if target == "" {
		return "", false
	}
	if key, ok := i.byKey[target]; ok {
		return key, true
	}
	if key, ok := i.byTitle[target]; ok {
		return key, true
	}
	if !strings.HasSuffix(target, ".md") {
		if key, ok := i.byKey[target+".md"]; ok {
			return key, true
		}
	}
	return "", false
}

func titleFromKey(key string) string {
	base := filepath.Base(key)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func sortedKeys(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
