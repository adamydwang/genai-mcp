package gemini

import (
	"context"
	"fmt"

	"genai-mcp/common"
	"genai-mcp/internal/oss"
)

// GeminiClient 实现 GenimiIface 接口的包装器
type GeminiClient struct {
	client *Client
}

// NewGeminiClientFromConfig 从配置创建 Gemini 客户端
func NewGeminiClientFromConfig(cfg *common.Config) (*GeminiClient, error) {
	config := Config{
		APIKey:           cfg.GenAIAPIKey,
		BaseURL:          cfg.GenAIBaseURL,
		ModelName:        cfg.GenAIModelName,
		OSSUploadEnabled: cfg.OSSUploadEnabled,
		OSSBucket:        cfg.OSSBucket,
	}

	// 如果启用了 OSS 上传，创建 OSS 客户端
	if cfg.OSSUploadEnabled {
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
func (g *GeminiClient) EditImage(ctx context.Context, prompt string, image_url string) (string, error) {
	return g.client.EditImage(ctx, prompt, image_url)
}

// Close 关闭客户端
func (g *GeminiClient) Close() error {
	if g.client != nil {
		return g.client.Close()
	}
	return nil
}
