package tools

import (
	"context"
	"fmt"

	"genai-mcp/internal/genai/gemini"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterGeminiTools 注册 Gemini 图片生成和编辑的 MCP tools
func RegisterGeminiTools(s *server.MCPServer, geminiClient gemini.GenimiIface) error {
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
			return mcp.NewToolResultError(fmt.Sprintf("prompt parameter is required: %v", err)), nil
		}

		// 调用 Gemini 生成图片
		imageURL, err := geminiClient.GenerateImage(ctx, prompt)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to generate image: %v", err)), nil
		}

		// 返回结果
		return mcp.NewToolResultText(fmt.Sprintf("Generated image: %s", imageURL)), nil
	})

	// 注册图片编辑工具
	editImageTool := mcp.NewTool(
		"gemini_edit_image",
		mcp.WithDescription("Edit an image using Gemini AI based on a text prompt. Takes an image URL and a prompt, returns the edited image URL or data URI."),
		mcp.WithString("prompt",
			mcp.Required(),
			mcp.Description("Text prompt describing how to edit the image"),
		),
		mcp.WithString("image_url",
			mcp.Required(),
			mcp.Description("URL or data URI of the image to edit"),
		),
	)

	s.AddTool(editImageTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// 获取参数
		prompt, err := req.RequireString("prompt")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("prompt parameter is required: %v", err)), nil
		}

		imageURL, err := req.RequireString("image_url")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("image_url parameter is required: %v", err)), nil
		}

		// 调用 Gemini 编辑图片
		editedImageURL, err := geminiClient.EditImage(ctx, prompt, imageURL)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to edit image: %v", err)), nil
		}

		// 返回结果
		return mcp.NewToolResultText(fmt.Sprintf("Edited image: %s", editedImageURL)), nil
	})

	return nil
}
