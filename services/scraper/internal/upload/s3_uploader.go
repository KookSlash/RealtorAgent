package upload

import (
	"context"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	appconfig "github.com/KookSlash/RealtorAgent/services/scraper/internal/config"
)

type Uploader interface {
	UploadFile(ctx context.Context, bucket, key, localPath, contentType string) (etag string, size int64, err error)
}

type s3PutObjectAPI interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

type S3Uploader struct {
	client s3PutObjectAPI
}

func NewS3Uploader(ctx context.Context, cfg appconfig.Config) (*S3Uploader, error) {
	awsCfg, err := loadAWSConfig(ctx, cfg.AWSRegion, cfg.LocalstackEndpoint)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		if usePathStyle(cfg.LocalstackEndpoint) {
			options.UsePathStyle = true
		}
	})

	return &S3Uploader{client: client}, nil
}

func NewS3UploaderFromClient(client s3PutObjectAPI) *S3Uploader {
	return &S3Uploader{client: client}
}

func (u *S3Uploader) UploadFile(ctx context.Context, bucket, key, localPath, contentType string) (string, int64, error) {
	file, err := os.Open(localPath)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return "", 0, err
	}

	input := &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   file,
	}
	if strings.TrimSpace(contentType) != "" {
		input.ContentType = aws.String(contentType)
	}

	output, err := u.client.PutObject(ctx, input)
	if err != nil {
		return "", 0, err
	}

	etag := ""
	if output != nil && output.ETag != nil {
		etag = aws.ToString(output.ETag)
	}

	return etag, info.Size(), nil
}

func loadAWSConfig(ctx context.Context, region, localstackEndpoint string) (aws.Config, error) {
	loadOptions := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}

	if strings.TrimSpace(localstackEndpoint) != "" {
		resolver := newS3EndpointResolver(localstackEndpoint)
		loadOptions = append(loadOptions,
			config.WithEndpointResolverWithOptions(resolver),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
		)
	}

	return config.LoadDefaultConfig(ctx, loadOptions...)
}

func newS3EndpointResolver(localstackEndpoint string) aws.EndpointResolverWithOptions {
	endpoint := strings.TrimSpace(localstackEndpoint)
	return aws.EndpointResolverWithOptionsFunc(func(service, region string, _ ...interface{}) (aws.Endpoint, error) {
		if service != s3.ServiceID {
			return aws.Endpoint{}, &aws.EndpointNotFoundError{}
		}
		return aws.Endpoint{
			URL:               endpoint,
			SigningRegion:     region,
			HostnameImmutable: true,
		}, nil
	})
}

func usePathStyle(localstackEndpoint string) bool {
	return strings.TrimSpace(localstackEndpoint) != ""
}
