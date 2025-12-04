package gemini

import (
	"context"
	"fmt"
	"strings"
	"time"

	"genai-mcp/common"
	"genai-mcp/internal/oss"
)

// GeminiClient 实现 GenimiIface 接口的包装器
type GeminiClient struct {
	client *Client
}

// NewGeminiClientFromConfig 从配置创建 Gemini 客户端
func NewGeminiClientFromConfig(cfg *common.Config) (*GeminiClient, error) {
	// 根据 GENAI_IMAGE_FORMAT 决定是否上传到 OSS
	// 当格式为 "url" 时，启用 OSS 上传；否则直接返回 base64/data URI
	ossUploadEnabled := strings.EqualFold(cfg.GenAIImageFormat, "url")

	config := Config{
		APIKey:            cfg.GenAIAPIKey,
		BaseURL:           cfg.GenAIBaseURL,
		GenerateModelName: cfg.GenAIGenModelName,
		EditModelName:     cfg.GenAIEditModelName,
		OSSUploadEnabled:  ossUploadEnabled,
		OSSBucket:         cfg.OSSBucket,
		ImageFormat:       cfg.GenAIImageFormat,
		Timeout:           time.Duration(cfg.GenAITimeoutSeconds) * time.Second,
	}

	// 如果启用了 OSS 上传，创建 OSS 客户端
	if ossUploadEnabled {
		ossClient, err := oss.NewOSSClientFromConfig(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create OSS client: %w", err)
		}
		config.OSSClient = ossClient
	}

	client, err := NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create gemini client: %w", err)
	}

	return &GeminiClient{
		client: client,
	}, nil
}

// GenerateImage 实现 GenimiIface 接口的文生图方法
func (g *GeminiClient) GenerateImage(ctx context.Context, prompt string) (string, error) {
	return g.client.GenerateImage(ctx, prompt)
}

// EditImage 实现 GenimiIface 接口的图片编辑方法
func (g *GeminiClient) EditImage(ctx context.Context, prompt string, image_urls []string) (string, error) {
	return g.client.EditImage(ctx, prompt, image_urls)
}

// Close 关闭客户端
func (g *GeminiClient) Close() error {
	if g.client != nil {
		return g.client.Close()
	}
	return nil
}
