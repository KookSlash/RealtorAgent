package vault

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type fakeS3 struct {
	copyInput *s3.CopyObjectInput
	putInput  *s3.PutObjectInput
}

func (f *fakeS3) CopyObject(ctx context.Context, params *s3.CopyObjectInput, _ ...func(*s3.Options)) (*s3.CopyObjectOutput, error) {
	f.copyInput = params
	return &s3.CopyObjectOutput{}, nil
}

func (f *fakeS3) PutObject(ctx context.Context, params *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	f.putInput = params
	return &s3.PutObjectOutput{}, nil
}

func TestBuildKeys(t *testing.T) {
	key := "raw/realtorca/2025-01-01/run-1.jsonl"
	keys := BuildKeys(key)

	if keys.Raw != "vault/raw/raw/realtorca/2025-01-01/run-1.jsonl" {
		t.Fatalf("unexpected raw key: %s", keys.Raw)
	}
	if keys.Normalized != "vault/normalized/raw/realtorca/2025-01-01/run-1.jsonl.normalized.jsonl" {
		t.Fatalf("unexpected normalized key: %s", keys.Normalized)
	}
	if keys.Errors != "vault/errors/raw/realtorca/2025-01-01/run-1.jsonl.errors.jsonl" {
		t.Fatalf("unexpected errors key: %s", keys.Errors)
	}
}

func TestBuildKeysLeadingSlash(t *testing.T) {
	key := "/raw/realtorca/2025-01-01/run-1.jsonl"
	keys := BuildKeys(key)

	if keys.Raw != "vault/raw/raw/realtorca/2025-01-01/run-1.jsonl" {
		t.Fatalf("unexpected raw key for leading slash: %s", keys.Raw)
	}
	if keys.Normalized != "vault/normalized/raw/realtorca/2025-01-01/run-1.jsonl.normalized.jsonl" {
		t.Fatalf("unexpected normalized key for leading slash: %s", keys.Normalized)
	}
	if keys.Errors != "vault/errors/raw/realtorca/2025-01-01/run-1.jsonl.errors.jsonl" {
		t.Fatalf("unexpected errors key for leading slash: %s", keys.Errors)
	}
}

func TestBuildKeysCleansPath(t *testing.T) {
	key := "raw//realtorca//2025-01-01//run-1.jsonl"
	keys := BuildKeys(key)

	if !strings.Contains(keys.Raw, "raw/realtorca/2025-01-01/run-1.jsonl") {
		t.Fatalf("expected cleaned raw key, got %s", keys.Raw)
	}
}

func TestCopyObjectUsesExpectedKeys(t *testing.T) {
	client := &fakeS3{}
	err := CopyObject(context.Background(), client, "raw-bucket", "raw/key.jsonl", "vault-bucket", "vault/raw/raw/key.jsonl")
	if err != nil {
		t.Fatalf("copy failed: %v", err)
	}
	if client.copyInput == nil {
		t.Fatalf("expected copy input to be set")
	}
	if got := *client.copyInput.Bucket; got != "vault-bucket" {
		t.Fatalf("expected bucket vault-bucket, got %s", got)
	}
	if got := *client.copyInput.Key; got != "vault/raw/raw/key.jsonl" {
		t.Fatalf("expected key vault/raw/raw/key.jsonl, got %s", got)
	}
	copySource := url.PathEscape(fmt.Sprintf("%s/%s", "raw-bucket", "raw/key.jsonl"))
	if got := *client.copyInput.CopySource; got != copySource {
		t.Fatalf("expected copy source %s, got %s", copySource, got)
	}
}

func TestPutObjectUsesExpectedKeys(t *testing.T) {
	client := &fakeS3{}
	body := strings.NewReader("payload")

	err := PutObject(context.Background(), client, "vault-bucket", "vault/normalized/key.jsonl", body)
	if err != nil {
		t.Fatalf("put failed: %v", err)
	}
	if client.putInput == nil {
		t.Fatalf("expected put input to be set")
	}
	if got := *client.putInput.Bucket; got != "vault-bucket" {
		t.Fatalf("expected bucket vault-bucket, got %s", got)
	}
	if got := *client.putInput.Key; got != "vault/normalized/key.jsonl" {
		t.Fatalf("expected key vault/normalized/key.jsonl, got %s", got)
	}
}
