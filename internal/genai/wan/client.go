package wan

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

// 默认请求超时时间（调用阿里百炼万相等接口）
const defaultWanTimeout = 60 * time.Second

// Client Wan 客户端实现，负责调用阿里百炼万相相关的图片接口。
//
// 注意：
// - 具体的 API 路径、请求/响应结构请参照阿里百炼控制台文档：
//   - 文生图：https://bailian.console.aliyun.com/?tab=api#/api/?type=model&url=2862677
//   - 图像编辑：https://bailian.console.aliyun.com/?tab=api#/api/?type=model&url=2982258
//   - 这里的实现使用通用的 JSON 结构和常见字段名（task_id / input / prompt 等），
//     如果与实际文档不一致，可以在不改动 WanIface 的前提下调整 Client 内部实现。
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

	// 可选：不同任务的相对路径（如果不配置则使用默认占位路径）
	generateCreatePath string
	generateQueryPath  string
	editCreatePath     string
	editQueryPath      string

	timeout time.Duration
}

// Config Wan 客户端配置。
//
// baseURL / apiKey / model 等字段应根据阿里百炼控制台中对应模型的「API 调用示例」进行配置。
// 例如：
//   - BaseURL: "https://dashscope.aliyuncs.com" 或 Bailian 提供的网关地址
//   - APIKey:  控制台发放的 API Key
//   - Model:   如 "wanx-v1" 或文档中对应的模型名
//
// 具体取值以阿里百炼文档为准。
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

// NewWanClientFromConfig 从通用配置创建 Wan 客户端。
// 仅当 common.Config.GenAIProvider=wan 时使用。
func NewWanClientFromConfig(cfg *common.Config) (*Client, error) {
	// 根据 GENAI_IMAGE_FORMAT 决定是否上传到 OSS
	// 当格式为 "url" 时，启用 OSS 上传；否则直接返回 base64 / 源 URL
	ossUploadEnabled := strings.EqualFold(cfg.GenAIImageFormat, "url")

	wanCfg := Config{
		// Wan 与 Gemini 共用 GENAI_BASE_URL / GENAI_API_KEY，两类任务分别使用不同模型
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
			return nil, fmt.Errorf("failed to create OSS client for Wan: %w", err)
		}
		wanCfg.OSSClient = ossClient
	}

	return NewClient(wanCfg)
}

// NewClient 创建 Wan 客户端。
func NewClient(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("wan base URL is required")
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("wan API key is required")
	}
	if cfg.GenModel == "" && cfg.EditModel == "" {
		return nil, fmt.Errorf("at least one of wan gen/edit model is required")
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultWanTimeout
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

	// 如果未显式配置路径，提供合理的占位默认值，便于后续在一个地方统一调整。
	if c.generateCreatePath == "" {
		// TODO: 根据阿里百炼文档更新为真实路径
		c.generateCreatePath = "/api/v1/services/aigc/text2image/image-synthesis"
	}
	if c.generateQueryPath == "" {
		// TODO: 根据阿里百炼文档更新为真实路径
		c.generateQueryPath = "/api/v1/tasks"
	}
	if c.editCreatePath == "" {
		// TODO: 根据阿里百炼文档更新为真实路径
		c.editCreatePath = "/api/v1/services/aigc/image2image/image-synthesis"
	}
	if c.editQueryPath == "" {
		// TODO: 根据阿里百炼文档更新为真实路径
		c.editQueryPath = "/api/v1/tasks"
	}

	return c, nil
}

// Close 预留关闭方法，当前未持有需要显式关闭的资源。
func (c *Client) Close() error {
	return nil
}

// CreateGenerateImageTask 调用文生图任务创建接口。
func (c *Client) CreateGenerateImageTask(ctx context.Context, prompt string, negative_prompt string) (string, error) {
	common.WithFields(map[string]interface{}{
		"model":           c.genModel,
		"prompt":          prompt,
		"negative_prompt": negative_prompt,
		"endpoint":        c.baseURL + c.generateCreatePath,
	}).Info("Creating Wan generate-image task")

	// 构建 input，negative_prompt 为空时不写入（其为可选参数）
	input := map[string]interface{}{
		"prompt": prompt,
	}
	if negative_prompt != "" {
		input["negative_prompt"] = negative_prompt
	}

	// 构建请求体，参考官方示例：
	// {
	//   "model": "wan2.2-t2i-flash",
	//   "input": { "prompt": "...", "negative_prompt": "..." },
	//   "parameters": { "size": "1024*1024", "n": 1 }
	// }
	// 这里保留 parameters 字段，提供一个合理的默认值，后续可根据需要扩展为可配置。
	payload := map[string]interface{}{
		"model": c.genModel,
		"input": input,
		"parameters": map[string]interface{}{
			"size": "1024*1024",
			"n":    1,
		},
	}

	// 根据官方示例，异步任务需要在 Header 中加入：
	//   X-DashScope-Async: enable
	extraHeaders := map[string]string{
		"X-DashScope-Async": "enable",
	}

	body, err := c.doRequest(ctx, http.MethodPost, c.generateCreatePath, payload, extraHeaders)
	if err != nil {
		return "", fmt.Errorf("failed to create generate image task: %w", err)
	}

	var resp createTaskResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		common.WithError(err).WithField("body", string(body)).Error("Failed to parse Wan generate-image create-task response")
		return "", fmt.Errorf("failed to parse create task response: %w", err)
	}

	if resp.Output.TaskID == "" {
		common.WithField("body", string(body)).Error("Wan create-task response missing task_id")
		return "", fmt.Errorf("wan create task response missing task_id")
	}

	return resp.Output.TaskID, nil
}

// QueryGenerateImageTask 查询文生图任务结果。
//
// 参考 DashScope 文档，查询接口为：
//
//	GET /api/v1/tasks/{task_id}
//
// 这里直接返回服务端的完整 JSON 字符串，调用方可以自行解析需要的字段
// （例如图片 URL、base64 编码、任务状态等）。如需只返回图片 URL，可在后续根据
// 阿里百炼文档调整解析逻辑。
func (c *Client) QueryGenerateImageTask(ctx context.Context, task_id string) (string, error) {
	common.WithFields(map[string]interface{}{
		"task_id":  task_id,
		"endpoint": c.baseURL + c.generateQueryPath + "/" + task_id,
	}).Info("Querying Wan generate-image task")

	// DashScope 查询任务使用 GET 且 task_id 在 URL 路径中
	queryPath := fmt.Sprintf("%s/%s", c.generateQueryPath, task_id)
	body, err := c.doRequest(ctx, http.MethodGet, queryPath, nil, nil)
	if err != nil {
		return "", fmt.Errorf("failed to query generate image task: %w", err)
	}

	// 根据 GENAI_IMAGE_FORMAT 对结果进行格式化（base64 / url），否则返回原始 JSON
	return c.formatImageQueryResult(ctx, body)
}

// CreateEditImageTask 调用图像编辑 / 多图融合任务创建接口。
// 对应 DashScope 单图编辑 / 多图融合接口：
//
//	POST /api/v1/services/aigc/image2image/image-synthesis
//	body:
//	{
//	  "model": "wan2.5-i2i-preview",
//	  "input": {
//	    "prompt": "...",
//	    "images": ["url1", "url2", ...]
//	  },
//	  "parameters": { "n": 1 }
//	}
func (c *Client) CreateEditImageTask(ctx context.Context, prompt string, image_urls []string) (string, error) {
	common.WithFields(map[string]interface{}{
		"model":      c.editModel,
		"prompt":     prompt,
		"image_urls": image_urls,
		"endpoint":   c.baseURL + c.editCreatePath,
	}).Info("Creating Wan edit-image task")

	// 构建 input，包含提示词和图片数组
	input := map[string]interface{}{
		"prompt": prompt,
		"images": image_urls,
	}

	// 保持与 DashScope 示例一致：仅控制输出图片数量 n（输入图片由 images 决定）
	payload := map[string]interface{}{
		"model": c.editModel,
		"input": input,
		"parameters": map[string]interface{}{
			"n": 1,
		},
	}

	// 图像编辑同样是异步任务，按照 DashScope 约定加上异步头
	extraHeaders := map[string]string{
		"X-DashScope-Async": "enable",
	}

	body, err := c.doRequest(ctx, http.MethodPost, c.editCreatePath, payload, extraHeaders)
	if err != nil {
		return "", fmt.Errorf("failed to create edit image task: %w", err)
	}

	var resp createTaskResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		common.WithError(err).WithField("body", string(body)).Error("Failed to parse Wan edit-image create-task response")
		return "", fmt.Errorf("failed to parse create task response: %w", err)
	}

	if resp.Output.TaskID == "" {
		common.WithField("body", string(body)).Error("Wan edit-image create-task response missing task_id")
		return "", fmt.Errorf("wan create edit image task response missing task_id")
	}

	return resp.Output.TaskID, nil
}

// QueryEditImageTask 查询图像编辑任务结果。
//
// DashScope 任务查询同样复用：
//
//	GET /api/v1/tasks/{task_id}
//
// 这里直接返回服务端的完整 JSON 字符串，调用方可以自行解析需要的字段
// （例如图片 URL、base64 编码、任务状态等）。如需只返回图片 URL，可在后续根据
// 阿里百炼文档调整解析逻辑。
func (c *Client) QueryEditImageTask(ctx context.Context, task_id string) (string, error) {
	common.WithFields(map[string]interface{}{
		"task_id":  task_id,
		"endpoint": c.baseURL + c.editQueryPath + "/" + task_id,
	}).Info("Querying Wan edit-image task")

	// DashScope 查询任务使用 GET 且 task_id 在 URL 路径中
	queryPath := fmt.Sprintf("%s/%s", c.editQueryPath, task_id)
	body, err := c.doRequest(ctx, http.MethodGet, queryPath, nil, nil)
	if err != nil {
		return "", fmt.Errorf("failed to query edit image task: %w", err)
	}

	// 复用同一套图片格式处理逻辑
	return c.formatImageQueryResult(ctx, body)
}

// doRequest 统一封装 HTTP 请求逻辑。
//
// - method:      GET / POST 等
// - path:        相对 baseURL 的路径，例如 "/api/v1/xxx"
// - body:        将被编码为 JSON；如果为 nil，则不发送请求体
// - extraHeader: 需要额外添加的 HTTP 头（例如 X-DashScope-Async）
//
// 认证方式：
// - 这里默认使用 "Authorization: Bearer <apiKey>"，对应当前阿里百炼文档中常见的风格。
// - 如果实际需要 "X-DashScope-Token" 或其它 Header，可以在这里统一调整。
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
	// 阿里百炼通常使用 Authorization Bearer 认证
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	// 附加额外头部（如 X-DashScope-Async: enable）
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
		}).Error("Wan API returned non-success status")
		return nil, fmt.Errorf("wan api error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// createTaskResponse 用于解析创建任务接口中常见的返回结构。
//
// 当前 DashScope 返回示例：
//
//	{
//	  "request_id": "...",
//	  "output": {
//	    "task_id": "09d7...",
//	    "task_status": "PENDING"
//	  }
//	}
type createTaskResponse struct {
	Output struct {
		TaskID string `json:"task_id"`
	} `json:"output"`
}

// wanTaskQueryResponse 解析 Wan 查询任务结果中的任务状态与图片 URL 信息。
// 实际字段名如有差异，可在不改动 WanIface 的前提下调整该结构体。
type wanTaskQueryResponse struct {
	Output *struct {
		TaskStatus string `json:"task_status,omitempty"`
		Results    []struct {
			URL   string `json:"url,omitempty"`
			Image string `json:"image_url,omitempty"`
			// 预留其它可能字段，例如 base64 数据等
		} `json:"results,omitempty"`
	} `json:"output,omitempty"`
	// 错误场景通常为顶层 code / message：
	// {
	//   "code": "InvalidApiKey",
	//   "message": "Invalid API-key provided.",
	//   "request_id": "..."
	// }
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// formatImageQueryResult 根据配置的图片格式（base64 / url）格式化 Wan 查询任务返回的 JSON。
// - 当格式为 base64 时：下载 results[0] 的图片 URL，转为 data URI 替换对应字段。
// - 当格式为 url 时：若配置了 OSS，则下载图片并上传到 OSS，使用 OSS URL 替换对应字段。
// - 如果无法找到图片 URL 或配置不完整，则返回原始 JSON。
func (c *Client) formatImageQueryResult(ctx context.Context, body []byte) (string, error) {
	// 未设置格式或格式未知时，直接返回原始 JSON
	if c.imageFormat == "" ||
		(!strings.EqualFold(c.imageFormat, "base64") && !strings.EqualFold(c.imageFormat, "url")) {
		return string(body), nil
	}

	var resp wanTaskQueryResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		// 解析失败时，为保持兼容，返回原始 JSON
		common.WithError(err).WithField("body", string(body)).Warn("Wan: failed to parse task query response for formatting")
		return string(body), nil
	}

	// 若没有 output 或没有 task_status，则视为非标准成功结果（可能是错误或中间状态），直接返回
	if resp.Output == nil || resp.Output.TaskStatus == "" {
		return string(body), nil
	}

	// 仅当任务已 SUCCEEDED 时才进行图片处理，其它状态（PENDING / RUNNING / FAILED 等）原样返回
	if !strings.EqualFold(resp.Output.TaskStatus, "SUCCEEDED") {
		return string(body), nil
	}

	if len(resp.Output.Results) == 0 {
		// 成功但没有 results，直接返回原始 JSON，以防止误判
		return string(body), nil
	}

	// 目前只处理第一个结果，后续如需支持多图可扩展
	result := &resp.Output.Results[0]
	imageURL := result.URL
	if imageURL == "" {
		imageURL = result.Image
	}
	if imageURL == "" {
		// 没有可用的图片 URL，直接返回
		return string(body), nil
	}

	// base64 输出：下载原图并转为 data URI
	if strings.EqualFold(c.imageFormat, "base64") {
		data, mimeType, err := utils.DownloadImageFromURL(ctx, imageURL)
		if err != nil {
			common.WithError(err).WithField("image_url", imageURL).Error("Wan: failed to download image for base64 formatting")
			return "", fmt.Errorf("failed to download image for base64 formatting: %w", err)
		}

		base64Data := base64.StdEncoding.EncodeToString(data)
		dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)

		// 将结果中的 URL 替换为 data URI
		result.URL = dataURI
		result.Image = dataURI
	} else if strings.EqualFold(c.imageFormat, "url") {
		// url 输出：将图片上传到 OSS，返回 OSS URL
		if !c.ossUploadEnabled || c.ossClient == nil || c.ossBucket == "" {
			common.WithFields(map[string]interface{}{
				"oss_enabled": c.ossUploadEnabled,
				"has_client":  c.ossClient != nil,
				"bucket":      c.ossBucket,
			}).Error("Wan: OSS is not properly configured but image format is set to 'url'")
			return "", fmt.Errorf("OSS is not configured but image format is set to 'url'")
		}

		ossURL, err := c.uploadImageToOSS(ctx, imageURL)
		if err != nil {
			common.WithError(err).WithField("image_url", imageURL).Error("Wan: failed to upload image to OSS")
			return "", fmt.Errorf("failed to upload image to OSS: %w", err)
		}

		result.URL = ossURL
		result.Image = ossURL
	}

	// 将修改后的结构重新编码为 JSON 字符串返回
	updated, err := json.Marshal(resp)
	if err != nil {
		common.WithError(err).Error("Wan: failed to marshal formatted task query response")
		return "", fmt.Errorf("failed to marshal formatted task query response: %w", err)
	}

	return string(updated), nil
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
	}).Debug("Wan: uploading image to OSS")

	reader := bytes.NewReader(data)
	url, err := c.ossClient.UploadFileWithURL(ctx, c.ossBucket, key, reader, mimeType, 3600*24*7)
	if err != nil {
		common.WithError(err).WithFields(map[string]interface{}{
			"bucket": c.ossBucket,
			"key":    key,
		}).Error("Wan: failed to upload image to OSS")
		return "", fmt.Errorf("failed to upload image to OSS: %w", err)
	}

	common.WithFields(map[string]interface{}{
		"bucket": c.ossBucket,
		"key":    key,
		"url":    url,
	}).Debug("Wan: image uploaded to OSS successfully")

	return url, nil
}
