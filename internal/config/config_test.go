package config

import (
	"strings"
	"testing"
)

func TestLoadS3ConfigReadsFiveLineEnv(t *testing.T) {
	input := strings.NewReader(strings.Join([]string{
		"s3.example.test",
		"ru-1",
		"access-key",
		"secret-key",
		"notes",
	}, "\n"))

	cfg, err := LoadS3Config(input)
	if err != nil {
		t.Fatalf("LoadS3Config returned error: %v", err)
	}

	if cfg.Endpoint != "https://s3.example.test" {
		t.Fatalf("Endpoint = %q, want https URL", cfg.Endpoint)
	}
	if cfg.Region != "ru-1" {
		t.Fatalf("Region = %q", cfg.Region)
	}
	if cfg.AccessKeyID != "access-key" {
		t.Fatalf("AccessKeyID = %q", cfg.AccessKeyID)
	}
	if cfg.SecretAccessKey != "secret-key" {
		t.Fatalf("SecretAccessKey = %q", cfg.SecretAccessKey)
	}
	if cfg.Bucket != "notes" {
		t.Fatalf("Bucket = %q", cfg.Bucket)
	}
}

func TestLoadServerConfigRequiresBearerToken(t *testing.T) {
	t.Setenv("NOTES_MCP_TOKEN", "")

	_, err := LoadServerConfig()
	if err == nil {
		t.Fatal("LoadServerConfig succeeded without NOTES_MCP_TOKEN")
	}
}

func TestLoadS3ConfigFromEnv(t *testing.T) {
	t.Setenv("NOTES_MCP_S3_ENDPOINT", "s3.example.test")
	t.Setenv("NOTES_MCP_S3_REGION", "ru-1")
	t.Setenv("NOTES_MCP_S3_ACCESS_KEY_ID", "access-key")
	t.Setenv("NOTES_MCP_S3_SECRET_ACCESS_KEY", "secret-key")
	t.Setenv("NOTES_MCP_S3_BUCKET", "notes")

	cfg, err := LoadS3ConfigFromEnv()
	if err != nil {
		t.Fatalf("LoadS3ConfigFromEnv returned error: %v", err)
	}
	if cfg.Endpoint != "https://s3.example.test" {
		t.Fatalf("Endpoint = %q", cfg.Endpoint)
	}
	if cfg.Bucket != "notes" {
		t.Fatalf("Bucket = %q", cfg.Bucket)
	}
}
