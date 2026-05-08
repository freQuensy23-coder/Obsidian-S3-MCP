package obsidian

import "testing"

func TestParseMarkdownExtractsWikilinksEmbedsAndTags(t *testing.T) {
	body := "#dataview #guide\nSee [[Haskell.md|Haskell]] and [[ML]].\n![[Materials/Pasted image.png]]\n"

	parsed := ParseMarkdown("Dataview получить список ссылок внутри.md", body)

	if parsed.Title != "Dataview получить список ссылок внутри" {
		t.Fatalf("Title = %q", parsed.Title)
	}
	if !contains(parsed.Tags, "dataview") || !contains(parsed.Tags, "guide") {
		t.Fatalf("Tags = %#v, want dataview and guide", parsed.Tags)
	}
	if !contains(parsed.Outlinks, "Haskell.md") || !contains(parsed.Outlinks, "ML") {
		t.Fatalf("Outlinks = %#v, want wikilinks", parsed.Outlinks)
	}
	if !contains(parsed.Embeds, "Materials/Pasted image.png") {
		t.Fatalf("Embeds = %#v, want embedded attachment", parsed.Embeds)
	}
}

func TestResolveWikilinkMatchesBasenameWithoutExtension(t *testing.T) {
	index := NewIndex([]NoteMeta{
		{Key: "Haskell.md", Title: "Haskell"},
		{Key: "Articles/LLM dating benchmark.md", Title: "LLM dating benchmark"},
	})

	key, ok := index.ResolveWikilink("LLM dating benchmark")
	if !ok {
		t.Fatal("ResolveWikilink did not find note by title")
	}
	if key != "Articles/LLM dating benchmark.md" {
		t.Fatalf("resolved key = %q", key)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
