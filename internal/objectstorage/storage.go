package objectstorage

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Object struct {
	Size        int64
	ContentType string
}

type Storage struct {
	client    *s3.Client
	presigner *s3.PresignClient
	bucket    string
	ttl       time.Duration
}

func (s *Storage) PresignGet(ctx context.Context, key string) (string, error) {
	result, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)}, func(options *s3.PresignOptions) { options.Expires = s.ttl })
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

func New(ctx context.Context, endpoint, publicEndpoint, region, bucket, accessKey, secretKey string, pathStyle bool, ttl time.Duration) (*Storage, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region), config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")))
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(cfg, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(endpoint)
		options.UsePathStyle = pathStyle
	})
	publicClient := s3.NewFromConfig(cfg, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(publicEndpoint)
		options.UsePathStyle = pathStyle
	})
	return &Storage{client: client, presigner: s3.NewPresignClient(publicClient), bucket: bucket, ttl: ttl}, nil
}

func (s *Storage) PresignPut(ctx context.Context, key, contentType string, size int64) (string, error) {
	result, err := s.presigner.PresignPutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key), ContentType: aws.String(contentType), ContentLength: aws.Int64(size)}, func(options *s3.PresignOptions) { options.Expires = s.ttl })
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

func (s *Storage) Head(ctx context.Context, key string) (Object, error) {
	result, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		return Object{}, err
	}
	return Object{Size: aws.ToInt64(result.ContentLength), ContentType: aws.ToString(result.ContentType)}, nil
}
