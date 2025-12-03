package oss

import (
	"bytes"
	"context"
	"fmt"
	"genai-mcp/common"
	"io"
	"net/http"
	"strings"
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
	common.WithFields(map[string]interface{}{
		"bucket":       bucket,
		"key":          key,
		"content_type": contentType,
	}).Debug("Starting file upload to OSS")

	// 读取文件内容
	body, err := io.ReadAll(reader)
	if err != nil {
		common.WithError(err).WithFields(map[string]interface{}{
			"bucket": bucket,
			"key":    key,
		}).Error("Failed to read file for upload")
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// 构建上传参数
	// 对于阿里云 OSS 等部分 S3 兼容服务，直接使用 SDK 的 PutObject
	// 会采用 aws-chunked 流式编码，导致
	// "aws-chunked encoding is not supported with the specified x-amz-content-sha256 value"
	// 这里改为：使用预签名的 PUT URL + 原生 HTTP 客户端上传，完全避开 aws-chunked。

	if strings.Contains(c.endpoint, ".aliyuncs.com") {
		common.WithFields(map[string]interface{}{
			"bucket": bucket,
			"key":    key,
		}).Debug("Using presigned PUT URL upload for Aliyun OSS")

		presignClient := s3.NewPresignClient(c.client)

		// 生成预签名 PUT URL
		reqCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()

		presigned, err := presignClient.PresignPutObject(reqCtx, &s3.PutObjectInput{
			Bucket:      aws.String(bucket),
			Key:         aws.String(key),
			ContentType: aws.String(contentType),
		})
		if err != nil {
			common.WithError(err).WithFields(map[string]interface{}{
				"bucket": bucket,
				"key":    key,
			}).Error("Failed to presign PUT URL for OSS upload")
			return "", fmt.Errorf("failed to presign PUT URL: %w", err)
		}

		// 使用预签名 URL 进行 HTTP PUT 上传（标准 Content-Length，无 aws-chunked）
		req, err := http.NewRequestWithContext(reqCtx, http.MethodPut, presigned.URL, bytes.NewReader(body))
		if err != nil {
			common.WithError(err).WithFields(map[string]interface{}{
				"bucket": bucket,
				"key":    key,
			}).Error("Failed to create HTTP request for OSS upload")
			return "", fmt.Errorf("failed to create HTTP request: %w", err)
		}

		// 设置预签名头部
		for k, v := range presigned.SignedHeader {
			for _, hv := range v {
				req.Header.Add(k, hv)
			}
		}

		// 确保 Content-Type 正确设置（如预签名中未包含）
		if contentType != "" && req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", contentType)
		}

		httpClient := &http.Client{
			Timeout: 60 * time.Second,
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			common.WithError(err).WithFields(map[string]interface{}{
				"bucket": bucket,
				"key":    key,
			}).Error("Failed to upload file to OSS via presigned PUT")
			return "", fmt.Errorf("failed to upload file via presigned PUT: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			common.WithFields(map[string]interface{}{
				"bucket":      bucket,
				"key":         key,
				"status_code": resp.StatusCode,
				"body":        string(respBody),
			}).Error("OSS presigned PUT upload returned non-2xx status")
			return "", fmt.Errorf("OSS upload failed: status code %d, body: %s", resp.StatusCode, string(respBody))
		}
	} else {
		// 标准 S3 或其他兼容服务：使用 SDK 的 PutObject
		input := &s3.PutObjectInput{
			Bucket:      aws.String(bucket),
			Key:         aws.String(key),
			Body:        bytes.NewReader(body),
			ContentType: aws.String(contentType),
		}

		// 执行上传
		_, err = c.client.PutObject(ctx, input)
		if err != nil {
			common.WithError(err).WithFields(map[string]interface{}{
				"bucket": bucket,
				"key":    key,
				"size":   len(body),
			}).Error("Failed to upload file to OSS")
			return "", fmt.Errorf("failed to upload file: %w", err)
		}
	}

	filePath := fmt.Sprintf("%s/%s", bucket, key)
	common.WithFields(map[string]interface{}{
		"bucket":    bucket,
		"key":       key,
		"file_path": filePath,
		"size":      len(body),
	}).Info("File uploaded to OSS successfully")

	// 返回文件路径（格式：bucket/key）
	return filePath, nil
}

// GetSignedURL 获取文件的带签名 URL
func (c *S3Client) GetSignedURL(ctx context.Context, bucket, key string, expiresIn int64) (string, error) {
	common.WithFields(map[string]interface{}{
		"bucket":     bucket,
		"key":        key,
		"expires_in": expiresIn,
	}).Debug("Generating signed URL for OSS file")

	presignClient := s3.NewPresignClient(c.client)

	request, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = time.Duration(expiresIn) * time.Second
	})

	if err != nil {
		common.WithError(err).WithFields(map[string]interface{}{
			"bucket": bucket,
			"key":    key,
		}).Error("Failed to generate signed URL")
		return "", fmt.Errorf("failed to presign URL: %w", err)
	}

	common.WithFields(map[string]interface{}{
		"bucket": bucket,
		"key":    key,
	}).Debug("Signed URL generated successfully")

	return request.URL, nil
}

// 上传文件并返回 URL（不再返回带签名 URL）
func (c *S3Client) UploadFileWithURL(ctx context.Context, bucket, key string, reader io.Reader, contentType string, expiresIn int64) (string, error) {
	// 先上传文件
	_, err := c.UploadFile(ctx, bucket, key, reader, contentType)
	if err != nil {
		return "", err
	}

	// 返回对象的普通访问 URL（非签名）
	return c.buildObjectURL(bucket, key), nil
}

// buildObjectURL 构造对象的公开 URL（不带签名）
func (c *S3Client) buildObjectURL(bucket, key string) string {
	// 优先使用自定义 endpoint（例如：oss-cn-beijing.aliyuncs.com）
	if c.endpoint != "" {
		return fmt.Sprintf("https://%s.%s/%s", bucket, c.endpoint, key)
	}

	// 如果知道 region，则使用区域化的 S3 域名
	if c.region != "" {
		return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", bucket, c.region, key)
	}

	// 回退到通用 S3 域名
	return fmt.Sprintf("https://%s.s3.amazonaws.com/%s", bucket, key)
}
