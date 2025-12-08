package apimart

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"genai-mcp/common"
	"genai-mcp/internal/oss"
	"genai-mcp/internal/utils"
)

// 默认请求超时时间（调用 APIMart 接口）
const defaultApimartTimeout = 60 * time.Second

// Client APIMart 客户端实现，负责调用 APIMart 相关的图片接口。
//
// 注意：
// - 具体的 API 路径、请求/响应结构请参照 APIMart 文档：
//   - 文生图：https://docs.apimart.ai/en/api-reference/images/gemini-3-pro/generation
//   - 任务查询：https://docs.apimart.ai/en/api-reference/task-management/get-task-status
type Client struct {
	httpClient *http.Client

	// 通用配置
	baseURL string
	apiKey  string
	// 分别用于图片生成与图片编辑的模型名称
	genModel  string
	editModel string

	// 图片输出与 OSS 配置（行为与 Gemini 对齐）
	ossClient        oss.OSSIface
	ossBucket        string
	ossUploadEnabled bool
	imageFormat      string // 图片输出格式: "base64" 或 "url"

	// API 路径
	generateCreatePath string
	generateQueryPath  string
	editCreatePath     string
	editQueryPath      string

	timeout time.Duration
}

// Config APIMart 客户端配置。
type Config struct {
	BaseURL string
	APIKey  string
	// 分别用于图片生成与图片编辑的模型名称
	GenModel  string
	EditModel string

	// 可选：OSS 与图片输出配置（与 Gemini 一致）
	OSSClient        oss.OSSIface
	OSSBucket        string
	OSSUploadEnabled bool
	ImageFormat      string

	// 可选：自定义各个任务的 HTTP 路径（相对 BaseURL）
	GenerateCreatePath string
	GenerateQueryPath  string
	EditCreatePath     string
	EditQueryPath      string

	Timeout time.Duration
}

// NewApimartClientFromConfig 从通用配置创建 APIMart 客户端。
// 仅当 common.Config.GenAIProvider=apimart 时使用。
func NewApimartClientFromConfig(cfg *common.Config) (*Client, error) {
	// 根据 GENAI_IMAGE_FORMAT 决定是否上传到 OSS
	// 当格式为 "url" 时，启用 OSS 上传；否则直接返回 base64 / 源 URL
	ossUploadEnabled := strings.EqualFold(cfg.GenAIImageFormat, "url")

	apimartCfg := Config{
		// APIMart 与 Gemini 共用 GENAI_BASE_URL / GENAI_API_KEY，两类任务分别使用不同模型
		BaseURL:   cfg.GenAIBaseURL,
		APIKey:    cfg.GenAIAPIKey,
		GenModel:  cfg.GenAIGenModelName,
		EditModel: cfg.GenAIEditModelName,
		Timeout:   time.Duration(cfg.GenAITimeoutSeconds) * time.Second,

		OSSUploadEnabled: ossUploadEnabled,
		OSSBucket:        cfg.OSSBucket,
		ImageFormat:      cfg.GenAIImageFormat,
	}

	// 如果启用了 OSS 上传，创建 OSS 客户端
	if ossUploadEnabled {
		ossClient, err := oss.NewOSSClientFromConfig(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create OSS client for APIMart: %w", err)
		}
		apimartCfg.OSSClient = ossClient
	}

	return NewClient(apimartCfg)
}

// NewClient 创建 APIMart 客户端。
func NewClient(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("apimart base URL is required")
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("apimart API key is required")
	}
	if cfg.GenModel == "" && cfg.EditModel == "" {
		return nil, fmt.Errorf("at least one of apimart gen/edit model is required")
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultApimartTimeout
	}

	// 如果只配置了一个模型，另一个复用它
	genModel := cfg.GenModel
	editModel := cfg.EditModel
	if genModel == "" {
		genModel = editModel
	}
	if editModel == "" {
		editModel = genModel
	}

	c := &Client{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		baseURL:            cfg.BaseURL,
		apiKey:             cfg.APIKey,
		genModel:           genModel,
		editModel:          editModel,
		generateCreatePath: cfg.GenerateCreatePath,
		generateQueryPath:  cfg.GenerateQueryPath,
		editCreatePath:     cfg.EditCreatePath,
		editQueryPath:      cfg.EditQueryPath,
		timeout:            timeout,
		ossClient:          cfg.OSSClient,
		ossBucket:          cfg.OSSBucket,
		ossUploadEnabled:   cfg.OSSUploadEnabled,
		imageFormat:        cfg.ImageFormat,
	}

	// 设置默认路径
	if c.generateCreatePath == "" {
		c.generateCreatePath = "/v1/images/generations"
	}
	if c.generateQueryPath == "" {
		c.generateQueryPath = "/v1/tasks"
	}
	if c.editCreatePath == "" {
		c.editCreatePath = "/v1/images/generations"
	}
	if c.editQueryPath == "" {
		c.editQueryPath = "/v1/tasks"
	}

	return c, nil
}

// Close 预留关闭方法，当前未持有需要显式关闭的资源。
func (c *Client) Close() error {
	return nil
}

// CreateGenerateImageTask 调用文生图任务创建接口。
func (c *Client) CreateGenerateImageTask(ctx context.Context, prompt string, size string, resolution string, n int) (string, error) {
	common.WithFields(map[string]interface{}{
		"model":      c.genModel,
		"prompt":     prompt,
		"size":       size,
		"resolution": resolution,
		"n":          n,
		"endpoint":   c.baseURL + c.generateCreatePath,
	}).Info("Creating APIMart generate-image task")

	// 构建请求体，参考 APIMart 文档：
	// {
	//   "model": "gemini-3-pro-image-preview",
	//   "prompt": "...",
	//   "size": "1:1",
	//   "n": 1,
	//   "resolution": "1K"
	// }
	payload := map[string]interface{}{
		"model":  c.genModel,
		"prompt": prompt,
	}

	// 可选参数
	if size != "" {
		payload["size"] = size
	}
	if resolution != "" {
		payload["resolution"] = resolution
	}
	if n > 0 {
		payload["n"] = n
	} else {
		payload["n"] = 1
	}

	body, err := c.doRequest(ctx, http.MethodPost, c.generateCreatePath, payload, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create generate image task: %w", err)
	}

	var resp createTaskResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		common.WithError(err).WithField("body", string(body)).Error("Failed to parse APIMart generate-image create-task response")
		return "", fmt.Errorf("failed to parse create task response: %w", err)
	}

	if len(resp.Data) == 0 || resp.Data[0].TaskID == "" {
		common.WithField("body", string(body)).Error("APIMart create-task response missing task_id")
		return "", fmt.Errorf("apimart create task response missing task_id")
	}

	return resp.Data[0].TaskID, nil
}

// QueryGenerateImageTask 查询文生图任务结果。
func (c *Client) QueryGenerateImageTask(ctx context.Context, task_id string) (string, error) {
	common.WithFields(map[string]interface{}{
		"task_id":  task_id,
		"endpoint": c.baseURL + c.generateQueryPath + "/" + task_id,
	}).Info("Querying APIMart generate-image task")

	// APIMart 查询任务使用 GET 且 task_id 在 URL 路径中
	queryPath := fmt.Sprintf("%s/%s", c.generateQueryPath, task_id)
	body, err := c.doRequest(ctx, http.MethodGet, queryPath, nil, nil)
	if err != nil {
		return "", fmt.Errorf("failed to query generate image task: %w", err)
	}

	var resp apimartTaskQueryResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("failed to parse generate task response: %w", err)
	}

	return c.formatImageResult(ctx, &resp)
}

// CreateEditImageTask 调用图像编辑任务创建接口。
func (c *Client) CreateEditImageTask(ctx context.Context, prompt string, image_urls []string, mask_url string) (string, error) {
	common.WithFields(map[string]interface{}{
		"model":      c.editModel,
		"prompt":     prompt,
		"image_urls": image_urls,
		"mask_url":   mask_url,
		"endpoint":   c.baseURL + c.editCreatePath,
	}).Info("Creating APIMart edit-image task")

	// 构建请求体，参考 APIMart 文档：
	// {
	//   "model": "gemini-3-pro-image-preview",
	//   "prompt": "...",
	//   "image_urls": ["url1", "url2", ...],
	//   "mask_url": "optional_mask_url"
	// }
	payload := map[string]interface{}{
		"model":      c.editModel,
		"prompt":     prompt,
		"image_urls": image_urls,
		"n":          1,
	}

	// 可选的蒙版图片
	if mask_url != "" {
		payload["mask_url"] = mask_url
	}

	body, err := c.doRequest(ctx, http.MethodPost, c.editCreatePath, payload, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create edit image task: %w", err)
	}

	var resp createTaskResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		common.WithError(err).WithField("body", string(body)).Error("Failed to parse APIMart edit-image create-task response")
		return "", fmt.Errorf("failed to parse create task response: %w", err)
	}

	if len(resp.Data) == 0 || resp.Data[0].TaskID == "" {
		common.WithField("body", string(body)).Error("APIMart edit-image create-task response missing task_id")
		return "", fmt.Errorf("apimart create edit image task response missing task_id")
	}

	return resp.Data[0].TaskID, nil
}

// QueryEditImageTask 查询图像编辑任务结果。
func (c *Client) QueryEditImageTask(ctx context.Context, task_id string) (string, error) {
	common.WithFields(map[string]interface{}{
		"task_id":  task_id,
		"endpoint": c.baseURL + c.editQueryPath + "/" + task_id,
	}).Info("Querying APIMart edit-image task")

	// APIMart 查询任务使用 GET 且 task_id 在 URL 路径中
	queryPath := fmt.Sprintf("%s/%s", c.editQueryPath, task_id)
	body, err := c.doRequest(ctx, http.MethodGet, queryPath, nil, nil)
	if err != nil {
		return "", fmt.Errorf("failed to query edit image task: %w", err)
	}

	var resp apimartTaskQueryResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("failed to parse edit task response: %w", err)
	}

	return c.formatImageResult(ctx, &resp)
}

// doRequest 统一封装 HTTP 请求逻辑。
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}, extraHeaders map[string]string) ([]byte, error) {
	url := c.baseURL + path

	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reader = bytes.NewReader(data)
	}

	// 为单次请求设置超时
	var cancel context.CancelFunc
	if _, ok := ctx.Deadline(); !ok && c.timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	// APIMart 使用 Authorization Bearer 认证
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	// 附加额外头部
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		common.WithFields(map[string]interface{}{
			"status_code": resp.StatusCode,
			"url":         url,
			"body":        string(respBody),
		}).Error("APIMart API returned non-success status")
		return nil, fmt.Errorf("apimart api error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// createTaskResponse 用于解析创建任务接口中常见的返回结构。
//
// APIMart 返回示例：
//
//	{
//	  "code": 200,
//	  "data": [
//	    {
//	      "status": "submitted",
//	      "task_id": "task_01K8SGYNNNVBQTXNR4MM964S7K"
//	    }
//	  ]
//	}
type createTaskResponse struct {
	Code int `json:"code"`
	Data []struct {
		Status string `json:"status"`
		TaskID string `json:"task_id"`
	} `json:"data"`
}

// apimartTaskQueryResponse 解析 APIMart 查询任务结果中的任务状态与图片 URL 信息。
type apimartTaskQueryResponse struct {
	Code int `json:"code"`
	Data *struct {
		Status   string `json:"status,omitempty"`
		Progress int    `json:"progress,omitempty"`
		Result   *struct {
			// Image generation results: result.images[].url is an array of URLs
			Images []struct {
				URL      []string `json:"url,omitempty"`
				ImageURL string   `json:"image_url,omitempty"` // fallback if present
			} `json:"images,omitempty"`
			// Legacy fallback fields
			ImageURL string `json:"image_url,omitempty"`
			URL      string `json:"url,omitempty"`
		} `json:"result,omitempty"`
		// Fallback results array (non-standard but kept for compatibility)
		Results []struct {
			ImageURL string `json:"image_url,omitempty"`
			URL      string `json:"url,omitempty"`
		} `json:"results,omitempty"`
	} `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
}

// formatImageResult 根据配置输出最终图片字符串（URL 或 base64 data URI）。
// 仅在任务已完成且找到图片时返回字符串；否则返回错误。
func (c *Client) formatImageResult(ctx context.Context, resp *apimartTaskQueryResponse) (string, error) {
	if resp == nil || resp.Data == nil || resp.Data.Status == "" {
		return "", fmt.Errorf("invalid task response: missing status")
	}

	status := strings.ToLower(resp.Data.Status)
	successStatuses := map[string]bool{
		"succeeded": true,
		"success":   true,
		"completed": true,
		"finished":  true,
		"done":      true,
	}
	if !successStatuses[status] {
		return "", fmt.Errorf("task not completed: status=%s", resp.Data.Status)
	}

	imageURL := extractFirstImageURL(resp)
	if imageURL == "" {
		return "", fmt.Errorf("task completed but image url is empty")
	}

	// base64 输出：下载原图并转为 data URI
	if strings.EqualFold(c.imageFormat, "base64") {
		data, mimeType, err := utils.DownloadImageFromURL(ctx, imageURL)
		if err != nil {
			common.WithError(err).WithField("image_url", imageURL).Error("APIMart: failed to download image for base64 formatting")
			return "", fmt.Errorf("failed to download image for base64 formatting: %w", err)
		}

		base64Data := base64.StdEncoding.EncodeToString(data)
		return fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data), nil
	}

	// url 输出：若开启 OSS 上传则返回 OSS URL，否则直接返回原图 URL
	if strings.EqualFold(c.imageFormat, "url") && c.ossUploadEnabled {
		if c.ossClient == nil || c.ossBucket == "" {
			common.WithFields(map[string]interface{}{
				"oss_enabled": c.ossUploadEnabled,
				"has_client":  c.ossClient != nil,
				"bucket":      c.ossBucket,
			}).Error("APIMart: OSS is not properly configured but image format is set to 'url'")
			return "", fmt.Errorf("OSS is not configured but image format is set to 'url'")
		}

		ossURL, err := c.uploadImageToOSS(ctx, imageURL)
		if err != nil {
			common.WithError(err).WithField("image_url", imageURL).Error("APIMart: failed to upload image to OSS")
			return "", fmt.Errorf("failed to upload image to OSS: %w", err)
		}
		return ossURL, nil
	}

	// 默认返回原始 URL
	return imageURL, nil
}

// extractFirstImageURL 提取任务结果中的首个图片 URL。
func extractFirstImageURL(resp *apimartTaskQueryResponse) string {
	if resp == nil {
		return ""
	}

	if resp.Data != nil && resp.Data.Result != nil && len(resp.Data.Result.Images) > 0 {
		img := resp.Data.Result.Images[0]
		if len(img.URL) > 0 && img.URL[0] != "" {
			return img.URL[0]
		}
		if img.ImageURL != "" {
			return img.ImageURL
		}
	}

	if resp.Data != nil && resp.Data.Result != nil {
		if resp.Data.Result.URL != "" {
			return resp.Data.Result.URL
		}
		if resp.Data.Result.ImageURL != "" {
			return resp.Data.Result.ImageURL
		}
	}

	if resp.Data != nil && len(resp.Data.Results) > 0 {
		if resp.Data.Results[0].URL != "" {
			return resp.Data.Results[0].URL
		}
		if resp.Data.Results[0].ImageURL != "" {
			return resp.Data.Results[0].ImageURL
		}
	}

	return ""
}

// uploadImageToOSS 将给定的 HTTP 图片 URL 下载后上传到 OSS，并返回 OSS URL。
func (c *Client) uploadImageToOSS(ctx context.Context, imageURL string) (string, error) {
	data, mimeType, err := utils.DownloadImageFromURL(ctx, imageURL)
	if err != nil {
		return "", fmt.Errorf("failed to download image from URL: %w", err)
	}

	path := utils.GenerateImagePath()
	fileName := utils.GenerateImageFileName(mimeType)
	key := fmt.Sprintf("%s%s", path, fileName)

	common.WithFields(map[string]interface{}{
		"bucket":       c.ossBucket,
		"key":          key,
		"content_type": mimeType,
		"size":         len(data),
	}).Debug("APIMart: uploading image to OSS")

	reader := bytes.NewReader(data)
	url, err := c.ossClient.UploadFileWithURL(ctx, c.ossBucket, key, reader, mimeType, 3600*24*7)
	if err != nil {
		common.WithError(err).WithFields(map[string]interface{}{
			"bucket": c.ossBucket,
			"key":    key,
		}).Error("APIMart: failed to upload image to OSS")
		return "", fmt.Errorf("failed to upload image to OSS: %w", err)
	}

	common.WithFields(map[string]interface{}{
		"bucket": c.ossBucket,
		"key":    key,
		"url":    url,
	}).Debug("APIMart: image uploaded to OSS successfully")

	return url, nil
}
