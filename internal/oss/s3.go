package oss

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Client S3 兼容的 OSS 客户端实现
type S3Client struct {
	client    *s3.Client
	endpoint  string
	region    string
	accessKey string
	secretKey string
}

// S3Config S3 客户端配置
type S3Config struct {
	Endpoint  string // OSS 服务端点，例如：s3.amazonaws.com 或 oss-cn-hangzhou.aliyuncs.com
	Region    string // 区域，例如：us-east-1 或 cn-hangzhou
	AccessKey string // Access Key ID
	SecretKey string // Secret Access Key
}

// NewS3Client 创建新的 S3 客户端
func NewS3Client(cfg S3Config) (*S3Client, error) {
	// 构建 AWS 配置选项
	opts := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKey,
			cfg.SecretKey,
			"",
		)),
	}

	// 如果提供了自定义端点（用于兼容其他 OSS 服务），使用自定义端点解析器
	if cfg.Endpoint != "" {
		customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:           fmt.Sprintf("https://%s", cfg.Endpoint),
				SigningRegion: cfg.Region,
			}, nil
		})
		opts = append(opts, config.WithEndpointResolverWithOptions(customResolver))
	}

	// 加载配置
	awsCfg, err := config.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// 创建 S3 客户端
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(fmt.Sprintf("https://%s", cfg.Endpoint))
		}
		// 使用路径样式（Path Style）以兼容更多 OSS 服务
		o.UsePathStyle = true
	})

	return &S3Client{
		client:    client,
		endpoint:  cfg.Endpoint,
		region:    cfg.Region,
		accessKey: cfg.AccessKey,
		secretKey: cfg.SecretKey,
	}, nil
}

// UploadFile 上传文件到 OSS
func (c *S3Client) UploadFile(ctx context.Context, bucket, key string, reader io.Reader, contentType string) (string, error) {
	// 读取文件内容
	body, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// 构建上传参数
	input := &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(body),
		ContentType: aws.String(contentType),
	}

	// 执行上传
	_, err = c.client.PutObject(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	// 返回文件路径（格式：bucket/key）
	return fmt.Sprintf("%s/%s", bucket, key), nil
}

// GetSignedURL 获取文件的带签名 URL
func (c *S3Client) GetSignedURL(ctx context.Context, bucket, key string, expiresIn int64) (string, error) {
	presignClient := s3.NewPresignClient(c.client)

	request, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = time.Duration(expiresIn) * time.Second
	})

	if err != nil {
		return "", fmt.Errorf("failed to presign URL: %w", err)
	}

	return request.URL, nil
}

// UploadFileWithSignedURL 上传文件并返回带签名的 URL
func (c *S3Client) UploadFileWithSignedURL(ctx context.Context, bucket, key string, reader io.Reader, contentType string, expiresIn int64) (string, error) {
	// 先上传文件
	_, err := c.UploadFile(ctx, bucket, key, reader, contentType)
	if err != nil {
		return "", err
	}

	// 获取签名 URL
	signedURL, err := c.GetSignedURL(ctx, bucket, key, expiresIn)
	if err != nil {
		return "", err
	}

	return signedURL, nil
}
