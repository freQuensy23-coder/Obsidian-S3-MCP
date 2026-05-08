package domain

import (
	"context"
	"errors"
	"time"
)

var ErrNotFound = errors.New("object not found")

type Object struct {
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
}

type Note struct {
	Object
	Title string `json:"title"`
	Body  string `json:"body,omitempty"`
}

type Overview struct {
	TotalObjects  int            `json:"total_objects"`
	MarkdownNotes int            `json:"markdown_notes"`
	TotalBytes    int64          `json:"total_bytes"`
	Folders       map[string]int `json:"folders"`
	Extensions    map[string]int `json:"extensions"`
}

type ObjectStore interface {
	ListObjects(ctx context.Context) ([]Object, error)
	GetObject(ctx context.Context, key string) ([]byte, error)
}

type VaultUseCase interface {
	Overview(context.Context) (Overview, error)
	ListNotes(context.Context) ([]Note, error)
	GetNote(context.Context, string) (Note, error)
	SearchNotes(context.Context, string, int) ([]Note, error)
	Backlinks(context.Context, string) ([]Note, error)
}
