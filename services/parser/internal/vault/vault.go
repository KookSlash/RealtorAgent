package vault

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"path"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3API interface {
	CopyObject(ctx context.Context, params *s3.CopyObjectInput, optFns ...func(*s3.Options)) (*s3.CopyObjectOutput, error)
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

type Keys struct {
	Raw        string
	Normalized string
	Errors     string
}

func BuildKeys(originalKey string) Keys {
	return Keys{
		Raw:        path.Join("vault", "raw", originalKey),
		Normalized: path.Join("vault", "normalized", originalKey) + ".normalized.jsonl",
		Errors:     path.Join("vault", "errors", originalKey) + ".errors.jsonl",
	}
}

func CopyObject(ctx context.Context, client S3API, sourceBucket, sourceKey, destBucket, destKey string) error {
	copySource := url.PathEscape(fmt.Sprintf("%s/%s", sourceBucket, sourceKey))
	_, err := client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(destBucket),
		Key:        aws.String(destKey),
		CopySource: aws.String(copySource),
	})
	return err
}

func PutObject(ctx context.Context, client S3API, bucket, key string, body io.Reader) error {
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   body,
	})
	return err
}
