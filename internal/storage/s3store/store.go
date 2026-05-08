package s3store

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"notesmcp/internal/config"
	"notesmcp/internal/domain"
)

type Store struct {
	client *s3.Client
	bucket string
}

func New(ctx context.Context, cfg config.S3Config) (*Store, error) {
	httpClient := http.DefaultClient
	if cfg.InsecureTLS {
		httpClient = &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}} //nolint:gosec
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, "")),
		awsconfig.WithHTTPClient(httpClient),
	)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(cfg.Endpoint)
		options.UsePathStyle = true
	})

	return &Store{client: client, bucket: cfg.Bucket}, nil
}

func (s *Store) ListObjects(ctx context.Context) ([]domain.Object, error) {
	var out []domain.Object
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{Bucket: aws.String(s.bucket)})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, object := range page.Contents {
			if object.Key == nil {
				continue
			}
			item := domain.Object{Key: *object.Key, Size: aws.ToInt64(object.Size)}
			if object.LastModified != nil {
				item.LastModified = *object.LastModified
			}
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *Store) GetObject(ctx context.Context, key string) ([]byte, error) {
	object, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer object.Body.Close()

	return io.ReadAll(object.Body)
}

func (s *Store) PutObject(ctx context.Context, key string, body []byte) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(body),
	})
	return err
}
