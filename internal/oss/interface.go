package oss

import (
	"context"
	"io"
)

// OSSIface OSS 客户端接口
type OSSIface interface {
	// UploadFile 上传文件到 OSS，返回文件路径
	UploadFile(ctx context.Context, bucket, key string, reader io.Reader, contentType string) (string, error)

	// GetSignedURL 获取文件的带签名 URL，用于临时访问
	// expiresIn 为过期时间（秒）
	GetSignedURL(ctx context.Context, bucket, key string, expiresIn int64) (string, error)

	// UploadFileWithURL 上传文件并返回 URL
	// 这是一个便捷方法，结合了 UploadFile 和 GetSignedURL
	UploadFileWithURL(ctx context.Context, bucket, key string, reader io.Reader, contentType string, expiresIn int64) (string, error)
}
