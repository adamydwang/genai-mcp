package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"genai-mcp/common"
	"genai-mcp/internal/genai/gemini"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterGeminiTools 注册 Gemini 图片生成和编辑的 MCP tools
func RegisterGeminiTools(s *server.MCPServer, geminiClient gemini.GenimiIface, modelName string) error {
	// 注册图片生成工具
	generateImageTool := mcp.NewTool(
		"gemini_generate_image",
		mcp.WithDescription("Generate an image using Gemini AI based on a text prompt. Returns the generated image URL or data URI."),
		mcp.WithString("prompt",
			mcp.Required(),
			mcp.Description("Text prompt describing the image to generate"),
		),
	)

	s.AddTool(generateImageTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// 获取参数
		prompt, err := req.RequireString("prompt")
		if err != nil {
			common.WithError(err).Error("Failed to get prompt parameter")
			return mcp.NewToolResultError(fmt.Sprintf("prompt parameter is required: %v", err)), nil
		}

		common.WithField("prompt", prompt).Info("Generating image with Gemini")

		// 调用 Gemini 生成图片
		imageURL, err := geminiClient.GenerateImage(ctx, prompt)
		if err != nil {
			common.WithError(err).WithField("prompt", prompt).Error("Failed to generate image")
			return mcp.NewToolResultError(fmt.Sprintf("failed to generate image: %v", err)), nil
		}

		// 日志中避免输出完整 base64 内容
		fields := map[string]interface{}{
			"prompt": prompt,
		}
		for k, v := range imageLogFields("image_url", imageURL) {
			fields[k] = v
		}
		common.WithFields(fields).Info("Image generated successfully")

		// 返回结果
		return mcp.NewToolResultText(fmt.Sprintf("Generated image: %s", imageURL)), nil
	})

	// 根据模型名生成 description
	maxImages := 1
	if modelName == "gemini-3-pro-image-preview" {
		maxImages = 14
	}
	editImageDescription := fmt.Sprintf("Edit images using Gemini AI based on a text prompt. Takes image URLs (array) and a prompt, returns the edited image URL or data URI. Model '%s' supports up to %d image(s).", modelName, maxImages)

	// 注册图片编辑工具
	editImageTool := mcp.NewTool(
		"gemini_edit_image",
		mcp.WithDescription(editImageDescription),
		mcp.WithString("prompt",
			mcp.Required(),
			mcp.Description("Text prompt describing how to edit the image"),
		),
		mcp.WithString("image_urls",
			mcp.Required(),
			mcp.Description("JSON array of image URLs or data URIs to edit. Example: [\"url1\", \"url2\"]"),
		),
	)

	s.AddTool(editImageTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// 获取参数
		prompt, err := req.RequireString("prompt")
		if err != nil {
			common.WithError(err).Error("Failed to get prompt parameter")
			return mcp.NewToolResultError(fmt.Sprintf("prompt parameter is required: %v", err)), nil
		}

		imageURLsJSON, err := req.RequireString("image_urls")
		if err != nil {
			common.WithError(err).Error("Failed to get image_urls parameter")
			return mcp.NewToolResultError(fmt.Sprintf("image_urls parameter is required: %v", err)), nil
		}

		// 解析 JSON 数组
		var imageURLs []string
		if err := json.Unmarshal([]byte(imageURLsJSON), &imageURLs); err != nil {
			common.WithError(err).WithField("image_urls", imageURLsJSON).Error("Failed to parse image_urls as JSON array")
			return mcp.NewToolResultError(fmt.Sprintf("image_urls must be a valid JSON array: %v", err)), nil
		}

		if len(imageURLs) == 0 {
			return mcp.NewToolResultError("image_urls array cannot be empty"), nil
		}

		fields := map[string]interface{}{
			"prompt":      prompt,
			"image_count": len(imageURLs),
		}
		common.WithFields(fields).Info("Editing image with Gemini")

		// 调用 Gemini 编辑图片
		editedImageURL, err := geminiClient.EditImage(ctx, prompt, imageURLs)
		if err != nil {
			errFields := map[string]interface{}{
				"prompt":      prompt,
				"image_count": len(imageURLs),
			}
			common.WithError(err).WithFields(errFields).Error("Failed to edit image")
			return mcp.NewToolResultError(fmt.Sprintf("failed to edit image: %v", err)), nil
		}

		successFields := map[string]interface{}{
			"prompt":      prompt,
			"image_count": len(imageURLs),
		}
		for k, v := range imageLogFields("edited_url", editedImageURL) {
			successFields[k] = v
		}
		common.WithFields(successFields).Info("Image edited successfully")

		// 返回结果（这里可以包含完整 base64 或 URL，因为这是返回给调用方，而不是日志）
		return mcp.NewToolResultText(fmt.Sprintf("Edited image: %s", editedImageURL)), nil
	})

	return nil
}

// imageLogFields 生成用于日志的图片字段，避免在日志中打印完整 base64 内容
// - 对于 data URI，仅记录是否为 data URI 以及长度
// - 对于普通 URL，记录完整 URL（通常为短链接或 OSS URL）
func imageLogFields(fieldName, ref string) map[string]interface{} {
	fields := map[string]interface{}{}
	if strings.HasPrefix(ref, "data:") {
		fields[fieldName+"_is_data_uri"] = true
		fields[fieldName+"_length"] = len(ref)
	} else if ref != "" {
		fields[fieldName] = ref
	}
	return fields
}
