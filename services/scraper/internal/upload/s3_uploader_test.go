package upload

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type fakeS3Client struct {
	input *s3.PutObjectInput
	body  []byte
	err   error
	etag  string
}

func (f *fakeS3Client) PutObject(_ context.Context, input *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	f.input = input
	if input != nil && input.Body != nil {
		data, _ := io.ReadAll(input.Body)
		f.body = data
	}
	if f.err != nil {
		return nil, f.err
	}
	return &s3.PutObjectOutput{ETag: aws.String(f.etag)}, nil
}

func TestUploadFile(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "payload.jsonl")
	content := []byte("{\"ok\":true}\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	fake := &fakeS3Client{etag: "\"abc123\""}
	uploader := NewS3UploaderFromClient(fake)

	etag, size, err := uploader.UploadFile(context.Background(), "bucket", "raw/realtorca/file.jsonl", path, "application/x-ndjson")
	if err != nil {
		t.Fatalf("upload file: %v", err)
	}
	if etag != "\"abc123\"" {
		t.Fatalf("unexpected etag: %s", etag)
	}
	if size != int64(len(content)) {
		t.Fatalf("unexpected size: %d", size)
	}
	if fake.input == nil {
		t.Fatalf("expected PutObject to be called")
	}
	if got := aws.ToString(fake.input.Bucket); got != "bucket" {
		t.Fatalf("unexpected bucket: %s", got)
	}
	if got := aws.ToString(fake.input.Key); got != "raw/realtorca/file.jsonl" {
		t.Fatalf("unexpected key: %s", got)
	}
	if got := aws.ToString(fake.input.ContentType); got != "application/x-ndjson" {
		t.Fatalf("unexpected content type: %s", got)
	}
	if string(fake.body) != string(content) {
		t.Fatalf("unexpected body: %s", string(fake.body))
	}
}

func TestEndpointResolverLocalstack(t *testing.T) {
	resolver := newS3EndpointResolver("http://localhost:4566")
	endpoint, err := resolver.ResolveEndpoint(s3.ServiceID, "us-west-2")
	if err != nil {
		t.Fatalf("resolve endpoint: %v", err)
	}
	if endpoint.URL != "http://localhost:4566" {
		t.Fatalf("unexpected endpoint url: %s", endpoint.URL)
	}
	if endpoint.SigningRegion != "us-west-2" {
		t.Fatalf("unexpected signing region: %s", endpoint.SigningRegion)
	}
	if !endpoint.HostnameImmutable {
		t.Fatalf("expected HostnameImmutable")
	}
}

func TestEndpointResolverNonS3(t *testing.T) {
	resolver := newS3EndpointResolver("http://localhost:4566")
	_, err := resolver.ResolveEndpoint("dynamodb", "us-west-2")
	var notFound *aws.EndpointNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("expected EndpointNotFoundError, got %v", err)
	}
}

func TestUsePathStyle(t *testing.T) {
	if usePathStyle("") {
		t.Fatalf("expected false for empty endpoint")
	}
	if !usePathStyle(" http://localhost:4566 ") {
		t.Fatalf("expected true for localstack endpoint")
	}
}
