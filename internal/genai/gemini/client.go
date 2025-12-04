package gemini

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"genai-mcp/common"
	"genai-mcp/internal/oss"
	"genai-mcp/internal/utils"

	"google.golang.org/genai"
)

// 默认请求超时时间（调用 Gemini 接口和相关网络请求）
const defaultGenAITimeout = 60 * time.Second

// Client Gemini 客户端实现
type Client struct {
	client *genai.Client
	// 分别用于生成与编辑的模型
	generateModel    string
	editModel        string
	ossClient        oss.OSSIface
	ossBucket        string
	ossUploadEnabled bool
	imageFormat      string // 图片输出格式: "base64" 或 "url"
	timeout          time.Duration
}

// Config Gemini 客户端配置
type Config struct {
	APIKey  string // API Key
	BaseURL string // 自定义 Base URL，如果为空则使用默认值
	// 分别用于图片生成与图片编辑的模型名称，例如：
	//   - 生成：gemini-3-pro-image-preview
	//   - 编辑：gemini-3-pro-image-preview
	GenerateModelName string // 文生图模型名称
	EditModelName     string // 图片编辑模型名称
	// OSS 配置（可选）
	OSSClient        oss.OSSIface  // OSS 客户端，如果启用上传则需要
	OSSBucket        string        // OSS 存储桶名称
	OSSUploadEnabled bool          // 是否启用 OSS 上传
	ImageFormat      string        // 图片输出格式: "base64" 或 "url"
	Timeout          time.Duration // 请求超时时间
}

// NewClient 创建新的 Gemini 客户端
func NewClient(cfg Config) (*Client, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	if cfg.GenerateModelName == "" && cfg.EditModelName == "" {
		return nil, fmt.Errorf("at least one of generate or edit model name is required")
	}

	// 构建客户端配置
	clientConfig := &genai.ClientConfig{
		APIKey:  cfg.APIKey,
		Backend: genai.BackendGeminiAPI,
	}

	// 如果提供了自定义 Base URL，设置 HTTPOptions
	if cfg.BaseURL != "" {
		clientConfig.HTTPOptions = genai.HTTPOptions{
			BaseURL: cfg.BaseURL,
		}
	}

	// 创建客户端
	client, err := genai.NewClient(context.Background(), clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}

	imageFormat := cfg.ImageFormat
	if imageFormat == "" {
		imageFormat = "base64" // 默认使用 base64
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultGenAITimeout
	}

	// 如果只配置了其中一个模型，另一个复用它，保持兼容
	generateModel := cfg.GenerateModelName
	editModel := cfg.EditModelName
	if generateModel == "" {
		generateModel = editModel
	}
	if editModel == "" {
		editModel = generateModel
	}

	return &Client{
		client:           client,
		generateModel:    generateModel,
		editModel:        editModel,
		ossClient:        cfg.OSSClient,
		ossBucket:        cfg.OSSBucket,
		ossUploadEnabled: cfg.OSSUploadEnabled,
		imageFormat:      imageFormat,
		timeout:          timeout,
	}, nil
}

// Close 关闭客户端（genai.Client 不需要显式关闭）
func (c *Client) Close() error {
	// genai.Client 不需要显式关闭
	return nil
}

// GenerateImage 文生图：根据文本提示生成图片
func (c *Client) GenerateImage(ctx context.Context, prompt string) (string, error) {
	common.WithFields(map[string]interface{}{
		"model":  c.generateModel,
		"prompt": prompt,
	}).Debug("Starting image generation")

	// 为本次请求设置超时时间，避免无休止等待
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// 构建请求内容
	parts := []*genai.Part{
		{Text: prompt},
	}

	// 调用 GenerateContent API
	result, err := c.client.Models.GenerateContent(ctx, c.generateModel, []*genai.Content{
		{Parts: parts},
	}, nil)
	if err != nil {
		common.WithError(err).WithFields(map[string]interface{}{
			"model":  c.generateModel,
			"prompt": prompt,
		}).Error("Failed to generate image from Gemini API")
		return "", fmt.Errorf("failed to generate image: %w", err)
	}

	// 从响应中提取图片 URL 或数据
	if len(result.Candidates) == 0 {
		return "", fmt.Errorf("no candidates in response")
	}

	candidate := result.Candidates[0]
	if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
		return "", fmt.Errorf("no content in candidate")
	}

	var imageResult string
	var imageData []byte
	var mimeType string

	for _, part := range candidate.Content.Parts {
		// 检查是否是内联图片数据
		if part.InlineData != nil {
			imageData = part.InlineData.Data
			mimeType = part.InlineData.MIMEType
			// 将图片数据编码为 base64
			base64Data := base64.StdEncoding.EncodeToString(part.InlineData.Data)
			imageResult = fmt.Sprintf("data:%s;base64,%s", part.InlineData.MIMEType, base64Data)
			break
		}

		// 检查是否是文件 URI
		if part.FileData != nil {
			imageResult = part.FileData.FileURI
			mimeType = part.FileData.MIMEType
			break
		}

		// 检查文本响应中是否包含 URL
		/*
			if part.Text != "" {
				imageResult = part.Text
				break
			}
		*/
	}

	if imageResult == "" {
		common.Error("No image data found in Gemini response")
		return "", fmt.Errorf("no image data found in response")
	}

	common.WithFields(map[string]interface{}{
		"model":        c.generateModel,
		"mime_type":    mimeType,
		"has_data":     len(imageData) > 0,
		"image_format": c.imageFormat,
	}).Debug("Image generated successfully")

	// 根据配置的图片格式处理结果
	return c.formatImageResult(ctx, imageResult, imageData, mimeType)
}

// EditImage 图片编辑：根据文本提示编辑图片
func (c *Client) EditImage(ctx context.Context, prompt string, imageURLs []string) (string, error) {
	// 验证图片数量
	maxImages := 1
	if c.editModel == "gemini-3-pro-image-preview" {
		maxImages = 14
	}

	if len(imageURLs) == 0 {
		return "", fmt.Errorf("at least one image URL is required")
	}

	if len(imageURLs) > maxImages {
		return "", fmt.Errorf("too many images: model %s supports at most %d images, got %d", c.editModel, maxImages, len(imageURLs))
	}

	common.WithFields(map[string]interface{}{
		"model":       c.editModel,
		"prompt":      prompt,
		"image_count": len(imageURLs),
		"image_urls":  imageURLs,
	}).Debug("Starting image editing")

	// 为本次请求设置超时时间，避免无休止等待
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// 构建请求内容：包含所有图片和编辑提示
	parts := make([]*genai.Part, 0, len(imageURLs)+1)

	// 处理所有图片并添加到 parts
	for i, imageURL := range imageURLs {
		var part *genai.Part

		// 判断是 data URI 还是 HTTP/HTTPS URL
		if strings.HasPrefix(imageURL, "data:") {
			// 处理 data URI：需要解析为 InlineData
			dataURIParts := strings.SplitN(imageURL, ",", 2)
			if len(dataURIParts) != 2 {
				return "", fmt.Errorf("invalid data URI format at index %d", i)
			}

			// 解析 MIME 类型
			mimePart := strings.TrimSuffix(dataURIParts[0], ";base64")
			mimeType := strings.TrimPrefix(mimePart, "data:")

			// 解码 base64 数据
			imageData, err := base64.StdEncoding.DecodeString(dataURIParts[1])
			if err != nil {
				common.WithError(err).WithFields(map[string]interface{}{
					"image_url": utils.TruncateForLog(imageURL, 100),
					"index":     i,
				}).Error("Failed to decode data URI")
				return "", fmt.Errorf("failed to decode data URI at index %d: %w", i, err)
			}

			common.WithFields(map[string]interface{}{
				"index":     i,
				"mime_type": mimeType,
				"size":      len(imageData),
				"type":      "data_uri",
			}).Debug("Processed data URI image")

			part = &genai.Part{
				InlineData: &genai.Blob{
					Data:     imageData,
					MIMEType: mimeType,
				},
			}
		} else if strings.HasPrefix(imageURL, "http://") || strings.HasPrefix(imageURL, "https://") {
			// 处理 HTTP/HTTPS URL：直接使用 FileData，让 Gemini API 自己获取
			mimeType := utils.InferMimeTypeFromURL(imageURL)

			common.WithFields(map[string]interface{}{
				"image_url": imageURL,
				"index":     i,
				"mime_type": mimeType,
				"type":      "http_url",
			}).Debug("Using HTTP URL directly (no download)")

			part = &genai.Part{
				FileData: &genai.FileData{
					FileURI:  imageURL,
					MIMEType: mimeType,
				},
			}
		} else {
			// 其他格式的 URL，尝试下载后使用 InlineData
			common.WithFields(map[string]interface{}{
				"image_url": imageURL,
				"index":     i,
			}).Debug("Downloading image (unsupported URL format)")

			imageData, mimeType, err := utils.DownloadImageFromURL(ctx, imageURL)
			if err != nil {
				common.WithError(err).WithFields(map[string]interface{}{
					"image_url": imageURL,
					"index":     i,
				}).Error("Failed to download image for editing")
				return "", fmt.Errorf("failed to download image at index %d: %w", i, err)
			}

			common.WithFields(map[string]interface{}{
				"image_url": imageURL,
				"index":     i,
				"mime_type": mimeType,
				"size":      len(imageData),
				"type":      "downloaded",
			}).Debug("Image downloaded successfully")

			part = &genai.Part{
				InlineData: &genai.Blob{
					Data:     imageData,
					MIMEType: mimeType,
				},
			}
		}

		parts = append(parts, part)
	}

	// 添加文本提示
	parts = append(parts, &genai.Part{Text: prompt})

	// 调用 GenerateContent API
	result, err := c.client.Models.GenerateContent(ctx, c.editModel, []*genai.Content{
		{Parts: parts},
	}, nil)
	if err != nil {
		common.WithError(err).WithFields(map[string]interface{}{
			"model":       c.editModel,
			"prompt":      prompt,
			"image_count": len(imageURLs),
		}).Error("Failed to edit image from Gemini API")
		return "", fmt.Errorf("failed to edit image: %w", err)
	}

	// 从响应中提取编辑后的图片
	if len(result.Candidates) == 0 {
		return "", fmt.Errorf("no candidates in response")
	}

	candidate := result.Candidates[0]
	if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
		return "", fmt.Errorf("no content in candidate")
	}

	var imageResult string
	var editedImageData []byte
	var editedMimeType string

	// 查找编辑后的图片数据
	for _, part := range candidate.Content.Parts {
		// 检查是否是内联图片数据
		if part.InlineData != nil {
			editedImageData = part.InlineData.Data
			editedMimeType = part.InlineData.MIMEType
			// 将图片数据编码为 base64
			base64Data := base64.StdEncoding.EncodeToString(part.InlineData.Data)
			imageResult = fmt.Sprintf("data:%s;base64,%s", part.InlineData.MIMEType, base64Data)
			break
		}

		// 检查是否是文件 URI
		if part.FileData != nil {
			imageResult = part.FileData.FileURI
			editedMimeType = part.FileData.MIMEType
			break
		}

		// 检查文本响应中是否包含 URL
		if part.Text != "" {
			imageResult = part.Text
			break
		}
	}

	if imageResult == "" {
		common.Error("No edited image data found in Gemini response")
		return "", fmt.Errorf("no edited image data found in response")
	}

	common.WithFields(map[string]interface{}{
		"model":        c.editModel,
		"mime_type":    editedMimeType,
		"has_data":     len(editedImageData) > 0,
		"image_format": c.imageFormat,
	}).Debug("Image edited successfully")

	// 根据配置的图片格式处理结果
	return c.formatImageResult(ctx, imageResult, editedImageData, editedMimeType)
}

// formatImageResult 根据配置的图片格式格式化结果
// imageResult: Gemini 返回的原始结果（可能是 data URI 或 URL）
// imageData: 如果 imageResult 是 data URI，这里包含原始数据；如果是 URL，则为 nil
// mimeType: 图片的 MIME 类型
func (c *Client) formatImageResult(ctx context.Context, imageResult string, imageData []byte, mimeType string) (string, error) {
	// 判断 imageResult 是 data URI 还是 URL
	isDataURI := strings.HasPrefix(imageResult, "data:")
	isHTTPURL := strings.HasPrefix(imageResult, "http://") || strings.HasPrefix(imageResult, "https://")

	if strings.EqualFold(c.imageFormat, "base64") {
		// 需要返回 base64 格式
		if isDataURI {
			// 已经是 data URI，直接返回
			return imageResult, nil
		} else {
			// 期望是 URL，需要下载并转换为 base64
			if !isHTTPURL {
				common.WithFields(map[string]interface{}{
					"is_data_uri": isDataURI,
					"is_http_url": isHTTPURL,
					"length":      len(imageResult),
				}).Error("Gemini result is not a valid image URL or data URI for base64 format")
				return "", fmt.Errorf("invalid image result: expected URL or data URI, got text")
			}

			common.Debug("Converting URL to base64 format")
			data, contentType, err := utils.DownloadImageFromURL(ctx, imageResult)
			if err != nil {
				common.WithError(err).Error("Failed to download image from URL for base64 conversion")
				return "", fmt.Errorf("failed to download image: %w", err)
			}
			// 转换为 base64 data URI
			base64Data := base64.StdEncoding.EncodeToString(data)
			return fmt.Sprintf("data:%s;base64,%s", contentType, base64Data), nil
		}
	} else if strings.EqualFold(c.imageFormat, "url") {
		// 需要返回 URL 格式（上传到 OSS）
		if !c.ossUploadEnabled || c.ossClient == nil || c.ossBucket == "" {
			return "", fmt.Errorf("OSS is not configured but image format is set to 'url'")
		}

		// 如果既不是 data URI 也不是 http(s) URL，则认为返回的不是图片
		if !isDataURI && !isHTTPURL {
			common.WithFields(map[string]interface{}{
				"is_data_uri": isDataURI,
				"is_http_url": isHTTPURL,
				"length":      len(imageResult),
			}).Error("Gemini result is not a valid image URL or data URI for URL format")
			return "", fmt.Errorf("invalid image result: expected URL or data URI, got text")
		}

		common.WithField("bucket", c.ossBucket).Info("Uploading image to OSS")
		uploadedURL, err := c.uploadImageToOSS(ctx, imageResult, imageData, mimeType)
		if err != nil {
			common.WithError(err).Error("Failed to upload image to OSS")
			return "", fmt.Errorf("failed to upload image to OSS: %w", err)
		}
		common.WithField("uploaded_url", uploadedURL).Info("Image uploaded to OSS successfully")
		return uploadedURL, nil
	} else {
		// 未知格式，返回原始结果
		common.Warnf("Unknown image format '%s', returning original result", c.imageFormat)
		return imageResult, nil
	}
}

// 保留旧函数名以兼容历史代码，同时转发到 common 包中的实现。
// TODO: 后续可直接使用 common.DownloadImageFromURL / common.InferMimeTypeFromURL，并删除这些包装。
func downloadImageFromURL(ctx context.Context, url string) ([]byte, string, error) {
	return utils.DownloadImageFromURL(ctx, url)
}

func inferMimeTypeFromURL(url string) string {
	return utils.InferMimeTypeFromURL(url)
}

// uploadImageToOSS 上传图片到 OSS
// imageResult 可能是 data URI 或 URL
// imageData 如果是 data URI，这里会包含原始数据；如果是 URL，则为 nil
// mimeType 图片的 MIME 类型
func (c *Client) uploadImageToOSS(ctx context.Context, imageResult string, imageData []byte, mimeType string) (string, error) {
	var data []byte
	var contentType string

	// 判断是 data URI 还是 URL
	if strings.HasPrefix(imageResult, "data:") {
		// 处理 data URI
		if imageData != nil {
			data = imageData
			contentType = mimeType
		} else {
			// 从 data URI 中解析数据
			parts := strings.SplitN(imageResult, ",", 2)
			if len(parts) != 2 {
				return "", fmt.Errorf("invalid data URI format")
			}
			// 解析 MIME 类型
			mimePart := strings.TrimSuffix(parts[0], ";base64")
			contentType = strings.TrimPrefix(mimePart, "data:")
			// 解码 base64 数据
			var err error
			data, err = base64.StdEncoding.DecodeString(parts[1])
			if err != nil {
				return "", fmt.Errorf("failed to decode base64 data: %w", err)
			}
		}
	} else {
		// 处理 URL，需要下载图片
		var err error
		data, contentType, err = utils.DownloadImageFromURL(ctx, imageResult)
		if err != nil {
			return "", fmt.Errorf("failed to download image from URL: %w", err)
		}
	}

	// 生成文件路径和名称
	path := utils.GenerateImagePath()
	fileName := utils.GenerateImageFileName(contentType)
	key := fmt.Sprintf("%s%s", path, fileName)

	common.WithFields(map[string]interface{}{
		"bucket":       c.ossBucket,
		"key":          key,
		"content_type": contentType,
		"size":         len(data),
	}).Debug("Uploading image to OSS")

	// 上传到 OSS
	reader := bytes.NewReader(data)
	signedURL, err := c.ossClient.UploadFileWithURL(ctx, c.ossBucket, key, reader, contentType, 3600*24*7) // 7天有效期
	if err != nil {
		common.WithError(err).WithFields(map[string]interface{}{
			"bucket": c.ossBucket,
			"key":    key,
		}).Error("Failed to upload image to OSS")
		return "", fmt.Errorf("failed to upload image to OSS: %w", err)
	}

	common.WithFields(map[string]interface{}{
		"bucket":     c.ossBucket,
		"key":        key,
		"signed_url": signedURL,
	}).Debug("Image uploaded to OSS successfully")

	return signedURL, nil
}
