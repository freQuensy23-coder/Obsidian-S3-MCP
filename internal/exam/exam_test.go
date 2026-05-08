package exam

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"notesmcp/internal/auth"
	"notesmcp/internal/config"
	"notesmcp/internal/handler/mcp"
	"notesmcp/internal/storage/cache"
	"notesmcp/internal/storage/s3store"
	"notesmcp/internal/usecase/vault"
)

const (
	minioAccessKey = "minioadmin"
	minioSecretKey = "minioadmin"
	testBucket     = "notes"
)

func TestMCPReadsObsidianVaultFromRealS3CompatibleBucket(t *testing.T) {
	ctx := context.Background()
	endpoint := startMinIO(t, ctx)
	s3Client := newS3Client(t, ctx, endpoint)
	createBucket(t, ctx, s3Client, testBucket)
	putObjects(t, ctx, s3Client, testBucket, map[string]string{
		"Home.md":             "[[Haskell]]\n![[Materials/image.png]]",
		"0000.Life.md":        "",
		"0001.ML.md":          "",
		"Haskell.md":          "#guide\nFunctional programming note",
		"Materials/image.png": "png-bytes",
		"Excalidraw/Board.md": "```compressed-json\nignored\n```",
		"Tasks/Read later.md": "todo",
	})

	store, err := s3store.New(ctx, config.S3Config{
		Endpoint:        endpoint,
		Region:          "us-east-1",
		AccessKeyID:     minioAccessKey,
		SecretAccessKey: minioSecretKey,
		Bucket:          testBucket,
	})
	if err != nil {
		t.Fatalf("create s3 store: %v", err)
	}

	service := vault.NewService(cache.New(store, 30*time.Second, time.Now))
	mcpServer := httptest.NewServer(auth.BearerAuth("test-token", mcp.NewHandler(service)))
	t.Cleanup(mcpServer.Close)

	overview := callTool(t, mcpServer.URL, "test-token", "vault_overview", map[string]any{})
	result, ok := overview["result"].(map[string]any)
	if !ok {
		t.Fatalf("overview result has unexpected shape: %#v", overview)
	}
	if got := result["total_objects"]; got != float64(7) {
		t.Fatalf("total_objects = %#v, want 7", got)
	}
	if got := result["markdown_notes"]; got != float64(6) {
		t.Fatalf("markdown_notes = %#v, want 6", got)
	}

	first := callTool(t, mcpServer.URL, "test-token", "get_note", map[string]any{"key": "Home.md"})
	firstResult := first["result"].(map[string]any)
	if firstResult["body"] != "[[Haskell]]\n![[Materials/image.png]]" {
		t.Fatalf("first Home.md body = %#v", firstResult["body"])
	}

	putObject(t, ctx, s3Client, testBucket, "Home.md", "changed in real minio")
	second := callTool(t, mcpServer.URL, "test-token", "get_note", map[string]any{"key": "Home.md"})
	secondResult := second["result"].(map[string]any)
	if secondResult["body"] != firstResult["body"] {
		t.Fatalf("cache missed inside TTL: second body = %#v, first body = %#v", secondResult["body"], firstResult["body"])
	}

	backlinks := callTool(t, mcpServer.URL, "test-token", "get_backlinks", map[string]any{"key": "Haskell.md"})
	backlinkResult := backlinks["result"].([]any)
	if len(backlinkResult) != 1 || backlinkResult[0].(map[string]any)["key"] != "Home.md" {
		t.Fatalf("backlinks = %#v, want Home.md", backlinkResult)
	}

	replace := callTool(t, mcpServer.URL, "test-token", "replace_in_note", map[string]any{
		"key":         "Tasks/Read later.md",
		"old_text":    "todo",
		"new_text":    "done",
		"replace_all": false,
	})
	if replace["result"].(map[string]any)["body"] != "done" {
		t.Fatalf("replace result = %#v, want done", replace["result"])
	}

	tagged := callTool(t, mcpServer.URL, "test-token", "add_note_tags", map[string]any{
		"key":  "Tasks/Read later.md",
		"tags": []string{"0000.Life.md", "[[0001.ML]]"},
	})
	if tagged["result"].(map[string]any)["body"] != "[[0000.Life]] [[0001.ML]]\ndone" {
		t.Fatalf("tagged body = %#v", tagged["result"].(map[string]any)["body"])
	}

	uploaded, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(testBucket),
		Key:    aws.String("Tasks/Read later.md"),
	})
	if err != nil {
		t.Fatalf("get written object from minio: %v", err)
	}
	defer uploaded.Body.Close()
	written, err := io.ReadAll(uploaded.Body)
	if err != nil {
		t.Fatalf("read written object: %v", err)
	}
	if string(written) != "[[0000.Life]] [[0001.ML]]\ndone" {
		t.Fatalf("minio body = %q", written)
	}
}

func startMinIO(t *testing.T, ctx context.Context) string {
	t.Helper()

	req := testcontainers.ContainerRequest{
		Image:        "minio/minio:RELEASE.2025-04-22T22-12-26Z",
		ExposedPorts: []string{"9000/tcp"},
		Env: map[string]string{
			"MINIO_ROOT_USER":     minioAccessKey,
			"MINIO_ROOT_PASSWORD": minioSecretKey,
		},
		Cmd:        []string{"server", "/data"},
		WaitingFor: wait.ForHTTP("/minio/health/ready").WithPort("9000/tcp").WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start minio container: %v", err)
	}
	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			t.Fatalf("terminate minio container: %v", err)
		}
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("minio host: %v", err)
	}
	port, err := container.MappedPort(ctx, "9000/tcp")
	if err != nil {
		t.Fatalf("minio mapped port: %v", err)
	}
	return fmt.Sprintf("http://%s:%s", host, port.Port())
}

func newS3Client(t *testing.T, ctx context.Context, endpoint string) *s3.Client {
	t.Helper()

	cfg, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(minioAccessKey, minioSecretKey, "")),
	)
	if err != nil {
		t.Fatalf("load aws config: %v", err)
	}

	return s3.NewFromConfig(cfg, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(endpoint)
		options.UsePathStyle = true
	})
}

func createBucket(t *testing.T, ctx context.Context, client *s3.Client, bucket string) {
	t.Helper()

	if _, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatalf("create bucket: %v", err)
	}
}

func putObjects(t *testing.T, ctx context.Context, client *s3.Client, bucket string, objects map[string]string) {
	t.Helper()

	for key, body := range objects {
		putObject(t, ctx, client, bucket, key, body)
	}
}

func putObject(t *testing.T, ctx context.Context, client *s3.Client, bucket, key, body string) {
	t.Helper()

	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   strings.NewReader(body),
	})
	if err != nil {
		t.Fatalf("put object %s: %v", key, err)
	}
}

func callTool(t *testing.T, url, token, name string, args map[string]any) map[string]any {
	t.Helper()

	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{"name": name, "arguments": args},
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out["error"] != nil {
		t.Fatalf("rpc error: %#v", out["error"])
	}
	return out
}
