package tools

import (
	"context"
	"fmt"
	"strings"

	"genai-mcp/common"
	"genai-mcp/internal/genai/wan"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterWanTools 注册阿里百炼「万相」相关的异步图片任务 MCP tools。
//
// 约定工具列表：
//   - wan_create_generate_image_task  文生图：创建异步任务，返回 task_id
//   - wan_query_generate_image_task   文生图：根据 task_id 查询任务结果，返回原始 JSON
//   - wan_create_edit_image_task      图像编辑：创建异步任务，返回 task_id
//   - wan_query_edit_image_task       图像编辑：根据 task_id 查询任务结果，返回原始 JSON
//
// WanIface 的具体实现由调用方创建（例如使用 internal/genai/wan/client.go）。
func RegisterWanTools(s *server.MCPServer, wanClient wan.WanIface) error {
	// 1. 文生图 - 创建任务
	createGenerateTool := mcp.NewTool(
		"wan_create_generate_image_task",
		mcp.WithDescription("Create an asynchronous image generation task using Ali Bailian Wanxiang. Returns a task_id."),
		mcp.WithString("prompt",
			mcp.Required(),
			mcp.Description("Text prompt describing the image to generate."),
		),
		mcp.WithString("negative_prompt",
			mcp.Description("Optional negative prompt to describe what should be avoided in the image."),
		),
	)

	s.AddTool(createGenerateTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		prompt, err := req.RequireString("prompt")
		if err != nil {
			common.WithError(err).Error("Wan: failed to get prompt parameter for create_generate_image_task")
			return mcp.NewToolResultError(fmt.Sprintf("prompt parameter is required: %v", err)), nil
		}

		// 可选参数：negative_prompt（如果未传则为空字符串）
		negativePrompt := ""

		common.WithFields(map[string]interface{}{
			"prompt":          prompt,
			"negative_prompt": negativePrompt,
		}).Info("Wan: creating generate-image task")

		taskID, err := wanClient.CreateGenerateImageTask(ctx, prompt, negativePrompt)
		if err != nil {
			common.WithError(err).WithFields(map[string]interface{}{
				"prompt":          prompt,
				"negative_prompt": negativePrompt,
			}).Error("Wan: failed to create generate-image task")
			return mcp.NewToolResultError(fmt.Sprintf("failed to create generate-image task: %v", err)), nil
		}

		common.WithFields(map[string]interface{}{
			"prompt":          prompt,
			"negative_prompt": negativePrompt,
			"task_id":         taskID,
		}).Info("Wan: generate-image task created successfully")

		return mcp.NewToolResultText(fmt.Sprintf("generate_image task_id: %s", taskID)), nil
	})

	// 2. 文生图 - 查询任务
	queryGenerateTool := mcp.NewTool(
		"wan_query_generate_image_task",
		mcp.WithDescription("Query the result of an image generation task created by Wanxiang using task_id. Returns raw JSON from the API."),
		mcp.WithString("task_id",
			mcp.Required(),
			mcp.Description("Task ID returned from wan_create_generate_image_task."),
		),
	)

	s.AddTool(queryGenerateTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		taskID, err := req.RequireString("task_id")
		if err != nil {
			common.WithError(err).Error("Wan: failed to get task_id parameter for query_generate_image_task")
			return mcp.NewToolResultError(fmt.Sprintf("task_id parameter is required: %v", err)), nil
		}

		common.WithField("task_id", taskID).Info("Wan: querying generate-image task")

		resultJSON, err := wanClient.QueryGenerateImageTask(ctx, taskID)
		if err != nil {
			common.WithError(err).WithField("task_id", taskID).Error("Wan: failed to query generate-image task")
			return mcp.NewToolResultError(fmt.Sprintf("failed to query generate-image task: %v", err)), nil
		}

		// 直接把 Wan 接口返回的 JSON 内容作为文本结果返回，由上层解析
		return mcp.NewToolResultText(resultJSON), nil
	})

	// 3. 图像编辑 - 创建任务
	createEditTool := mcp.NewTool(
		"wan_create_edit_image_task",
		mcp.WithDescription("Create an asynchronous image editing task using Ali Bailian Wanxiang. Returns a task_id."),
		mcp.WithString("prompt",
			mcp.Required(),
			mcp.Description("Text prompt describing how to edit the image."),
		),
		mcp.WithString("image_url",
			mcp.Required(),
			mcp.Description("HTTP/HTTPS URL of the source image to be edited. Wan only supports image URLs, not base64 or data URIs."),
		),
	)

	s.AddTool(createEditTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		prompt, err := req.RequireString("prompt")
		if err != nil {
			common.WithError(err).Error("Wan: failed to get prompt parameter for create_edit_image_task")
			return mcp.NewToolResultError(fmt.Sprintf("prompt parameter is required: %v", err)), nil
		}

		imageURL, err := req.RequireString("image_url")
		if err != nil {
			common.WithError(err).Error("Wan: failed to get image_url parameter for create_edit_image_task")
			return mcp.NewToolResultError(fmt.Sprintf("image_url parameter is required: %v", err)), nil
		}

		// Wan 只支持图片 URL 输入：必须是 http 或 https
		if !strings.HasPrefix(imageURL, "http://") && !strings.HasPrefix(imageURL, "https://") {
			common.WithField("image_url", imageURL).Error("Wan: image_url must be an HTTP/HTTPS URL (no base64 or data URIs)")
			return mcp.NewToolResultError("image_url must be an HTTP/HTTPS URL; Wan does not support base64 or data URIs"), nil
		}

		common.WithFields(map[string]interface{}{
			"prompt":    prompt,
			"image_url": imageURL,
		}).Info("Wan: creating edit-image task")

		// MCP 工具目前仍只接受单个 image_url，这里用单元素切片适配底层多图接口
		taskID, err := wanClient.CreateEditImageTask(ctx, prompt, []string{imageURL})
		if err != nil {
			common.WithError(err).WithFields(map[string]interface{}{
				"prompt":    prompt,
				"image_url": imageURL,
			}).Error("Wan: failed to create edit-image task")
			return mcp.NewToolResultError(fmt.Sprintf("failed to create edit-image task: %v", err)), nil
		}

		common.WithFields(map[string]interface{}{
			"prompt":    prompt,
			"image_url": imageURL,
			"task_id":   taskID,
		}).Info("Wan: edit-image task created successfully")

		return mcp.NewToolResultText(fmt.Sprintf("edit_image task_id: %s", taskID)), nil
	})

	// 4. 图像编辑 - 查询任务
	queryEditTool := mcp.NewTool(
		"wan_query_edit_image_task",
		mcp.WithDescription("Query the result of an image editing task created by Wanxiang using task_id. Returns raw JSON from the API."),
		mcp.WithString("task_id",
			mcp.Required(),
			mcp.Description("Task ID returned from wan_create_edit_image_task."),
		),
	)

	s.AddTool(queryEditTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		taskID, err := req.RequireString("task_id")
		if err != nil {
			common.WithError(err).Error("Wan: failed to get task_id parameter for query_edit_image_task")
			return mcp.NewToolResultError(fmt.Sprintf("task_id parameter is required: %v", err)), nil
		}

		common.WithField("task_id", taskID).Info("Wan: querying edit-image task")

		resultJSON, err := wanClient.QueryEditImageTask(ctx, taskID)
		if err != nil {
			common.WithError(err).WithField("task_id", taskID).Error("Wan: failed to query edit-image task")
			return mcp.NewToolResultError(fmt.Sprintf("failed to query edit-image task: %v", err)), nil
		}

		return mcp.NewToolResultText(resultJSON), nil
	})

	return nil
}
