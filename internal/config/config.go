package config

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

type S3Config struct {
	Endpoint        string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	InsecureTLS     bool
}

type ServerConfig struct {
	Addr  string
	Token string
}

func LoadS3Config(r io.Reader) (S3Config, error) {
	scanner := bufio.NewScanner(r)
	values := make([]string, 0, 5)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		values = append(values, line)
	}
	if err := scanner.Err(); err != nil {
		return S3Config{}, err
	}
	if len(values) != 5 {
		return S3Config{}, fmt.Errorf("expected 5 non-empty config lines, got %d", len(values))
	}

	endpoint := values[0]
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		endpoint = "https://" + endpoint
	}

	return S3Config{
		Endpoint:        endpoint,
		Region:          values[1],
		AccessKeyID:     values[2],
		SecretAccessKey: values[3],
		Bucket:          values[4],
		InsecureTLS:     parseBool(os.Getenv("NOTES_MCP_S3_INSECURE_SKIP_VERIFY")),
	}, nil
}

func LoadS3ConfigFile(path string) (S3Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return S3Config{}, err
	}
	defer file.Close()

	return LoadS3Config(file)
}

func LoadServerConfig() (ServerConfig, error) {
	token := strings.TrimSpace(os.Getenv("NOTES_MCP_TOKEN"))
	if token == "" {
		return ServerConfig{}, errors.New("NOTES_MCP_TOKEN is required")
	}

	addr := strings.TrimSpace(os.Getenv("NOTES_MCP_ADDR"))
	if addr == "" {
		addr = "127.0.0.1:8080"
	}

	return ServerConfig{Addr: addr, Token: token}, nil
}

func parseBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y":
		return true
	default:
		return false
	}
}
