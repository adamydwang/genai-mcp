package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"genai-mcp/common"
	"genai-mcp/internal/genai/apimart"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterApimartTools 注册 APIMart 相关的异步图片任务 MCP tools。
//
// 约定工具列表：
//   - apimart_create_generate_image_task  文生图：创建异步任务，返回 task_id
//   - apimart_query_generate_image_task   文生图：根据 task_id 查询任务结果，返回原始 JSON
//   - apimart_create_edit_image_task      图像编辑：创建异步任务，返回 task_id
//   - apimart_query_edit_image_task       图像编辑：根据 task_id 查询任务结果，返回原始 JSON
func RegisterApimartTools(s *server.MCPServer, apimartClient apimart.ApimartIface) error {
	// 1. 文生图 - 创建任务
	createGenerateTool := mcp.NewTool(
		"apimart_create_generate_image_task",
		mcp.WithDescription("Create an asynchronous image generation task using APIMart. Returns a task_id."),
		mcp.WithString("prompt",
			mcp.Required(),
			mcp.Description("Text prompt describing the image to generate."),
		),
		mcp.WithString("size",
			mcp.Description("Image generation size. Supported formats: 1:1, 2:3, 3:2, 3:4, 4:3, 4:5, 5:4, 9:16, 16:9, 21:9"),
		),
		mcp.WithString("resolution",
			mcp.Description("Output image resolution. Supported values: 1K (default), 2K, 4K"),
		),
		mcp.WithString("n",
			mcp.Description("Number of images to generate. Fixed at 1."),
		),
	)

	s.AddTool(createGenerateTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		prompt, err := req.RequireString("prompt")
		if err != nil {
			common.WithError(err).Error("APIMart: failed to get prompt parameter for create_generate_image_task")
			return mcp.NewToolResultError(fmt.Sprintf("prompt parameter is required: %v", err)), nil
		}

		// 可选参数
		size := req.GetString("size", "")
		resolution := req.GetString("resolution", "")
		n := req.GetInt("n", 1)
		if n <= 0 {
			n = 1
		}

		common.WithFields(map[string]interface{}{
			"prompt":     prompt,
			"size":       size,
			"resolution": resolution,
			"n":          n,
		}).Info("APIMart: creating generate-image task")

		taskID, err := apimartClient.CreateGenerateImageTask(ctx, prompt, size, resolution, n)
		if err != nil {
			common.WithError(err).WithFields(map[string]interface{}{
				"prompt":     prompt,
				"size":       size,
				"resolution": resolution,
				"n":          n,
			}).Error("APIMart: failed to create generate-image task")
			return mcp.NewToolResultError(fmt.Sprintf("failed to create generate-image task: %v", err)), nil
		}

		common.WithFields(map[string]interface{}{
			"prompt":     prompt,
			"size":       size,
			"resolution": resolution,
			"n":          n,
			"task_id":    taskID,
		}).Info("APIMart: generate-image task created successfully")

		return mcp.NewToolResultText(fmt.Sprintf("generate_image task_id: %s", taskID)), nil
	})

	// 2. 文生图 - 查询任务
	queryGenerateTool := mcp.NewTool(
		"apimart_query_generate_image_task",
		mcp.WithDescription("Query the result of an image generation task created by APIMart using task_id. Returns raw JSON from the API."),
		mcp.WithString("task_id",
			mcp.Required(),
			mcp.Description("Task ID returned from apimart_create_generate_image_task."),
		),
	)

	s.AddTool(queryGenerateTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		taskID, err := req.RequireString("task_id")
		if err != nil {
			common.WithError(err).Error("APIMart: failed to get task_id parameter for query_generate_image_task")
			return mcp.NewToolResultError(fmt.Sprintf("task_id parameter is required: %v", err)), nil
		}

		common.WithField("task_id", taskID).Info("APIMart: querying generate-image task")

		resultJSON, err := apimartClient.QueryGenerateImageTask(ctx, taskID)
		if err != nil {
			// 未完成任务，不视为错误，返回状态提示，便于上层继续轮询
			if strings.Contains(strings.ToLower(err.Error()), "not completed") {
				common.WithFields(map[string]interface{}{
					"task_id": taskID,
					"status":  err.Error(),
				}).Info("APIMart: generate-image task not completed yet")
				return mcp.NewToolResultText(err.Error()), nil
			}
			common.WithError(err).WithField("task_id", taskID).Error("APIMart: failed to query generate-image task")
			return mcp.NewToolResultError(fmt.Sprintf("failed to query generate-image task: %v", err)), nil
		}

		// 直接把 APIMart 接口返回的 JSON 内容作为文本结果返回，由上层解析
		return mcp.NewToolResultText(resultJSON), nil
	})

	// 3. 图像编辑 - 创建任务
	createEditTool := mcp.NewTool(
		"apimart_create_edit_image_task",
		mcp.WithDescription("Create an asynchronous image editing task using APIMart. Returns a task_id. Supports image URLs or base64 data URIs."),
		mcp.WithString("prompt",
			mcp.Required(),
			mcp.Description("Text prompt describing how to edit the image."),
		),
		mcp.WithString("image_urls",
			mcp.Required(),
			mcp.Description("JSON array of image URLs or base64 data URIs to edit. Example: [\"url1\", \"url2\"] or [\"data:image/jpeg;base64,...\"]"),
		),
		mcp.WithString("mask_url",
			mcp.Description("Optional mask image URL (PNG format). Size must match reference image. Must not exceed 4MB."),
		),
	)

	s.AddTool(createEditTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		prompt, err := req.RequireString("prompt")
		if err != nil {
			common.WithError(err).Error("APIMart: failed to get prompt parameter for create_edit_image_task")
			return mcp.NewToolResultError(fmt.Sprintf("prompt parameter is required: %v", err)), nil
		}

		imageURLsJSON, err := req.RequireString("image_urls")
		if err != nil {
			common.WithError(err).Error("APIMart: failed to get image_urls parameter for create_edit_image_task")
			return mcp.NewToolResultError(fmt.Sprintf("image_urls parameter is required: %v", err)), nil
		}

		// 解析 JSON 数组
		var imageURLs []string
		if err := json.Unmarshal([]byte(imageURLsJSON), &imageURLs); err != nil {
			common.WithError(err).WithField("image_urls", imageURLsJSON).Error("APIMart: failed to parse image_urls as JSON array")
			return mcp.NewToolResultError(fmt.Sprintf("image_urls must be a valid JSON array: %v", err)), nil
		}

		if len(imageURLs) == 0 {
			return mcp.NewToolResultError("image_urls array cannot be empty"), nil
		}

		// 可选参数：mask_url
		maskURL := req.GetString("mask_url", "")

		common.WithFields(map[string]interface{}{
			"prompt":      prompt,
			"image_count": len(imageURLs),
			"mask_url":    maskURL,
		}).Info("APIMart: creating edit-image task")

		taskID, err := apimartClient.CreateEditImageTask(ctx, prompt, imageURLs, maskURL)
		if err != nil {
			common.WithError(err).WithFields(map[string]interface{}{
				"prompt":      prompt,
				"image_count": len(imageURLs),
				"mask_url":    maskURL,
			}).Error("APIMart: failed to create edit-image task")
			return mcp.NewToolResultError(fmt.Sprintf("failed to create edit-image task: %v", err)), nil
		}

		common.WithFields(map[string]interface{}{
			"prompt":      prompt,
			"image_count": len(imageURLs),
			"mask_url":    maskURL,
			"task_id":     taskID,
		}).Info("APIMart: edit-image task created successfully")

		return mcp.NewToolResultText(fmt.Sprintf("edit_image task_id: %s", taskID)), nil
	})

	// 4. 图像编辑 - 查询任务
	queryEditTool := mcp.NewTool(
		"apimart_query_edit_image_task",
		mcp.WithDescription("Query the result of an image editing task created by APIMart using task_id. Returns raw JSON from the API."),
		mcp.WithString("task_id",
			mcp.Required(),
			mcp.Description("Task ID returned from apimart_create_edit_image_task."),
		),
	)

	s.AddTool(queryEditTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		taskID, err := req.RequireString("task_id")
		if err != nil {
			common.WithError(err).Error("APIMart: failed to get task_id parameter for query_edit_image_task")
			return mcp.NewToolResultError(fmt.Sprintf("task_id parameter is required: %v", err)), nil
		}

		common.WithField("task_id", taskID).Info("APIMart: querying edit-image task")

		resultJSON, err := apimartClient.QueryEditImageTask(ctx, taskID)
		if err != nil {
			// 未完成任务，不视为错误，返回状态提示，便于上层继续轮询
			if strings.Contains(strings.ToLower(err.Error()), "not completed") {
				common.WithFields(map[string]interface{}{
					"task_id": taskID,
					"status":  err.Error(),
				}).Info("APIMart: edit-image task not completed yet")
				return mcp.NewToolResultText(err.Error()), nil
			}
			common.WithError(err).WithField("task_id", taskID).Error("APIMart: failed to query edit-image task")
			return mcp.NewToolResultError(fmt.Sprintf("failed to query edit-image task: %v", err)), nil
		}

		return mcp.NewToolResultText(resultJSON), nil
	})

	return nil
}
