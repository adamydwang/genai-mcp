package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"genai-mcp/common"
	"genai-mcp/internal/genai/apimart"
	"genai-mcp/internal/genai/gemini"
	"genai-mcp/internal/genai/wan"
	"genai-mcp/internal/tools"

	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// 加载配置（会自动初始化日志系统）
	config, err := common.LoadConfig()
	if err != nil {
		common.Fatalf("Failed to load config: %v", err)
	}

	// 记录启动信息
	common.Info("Starting GenAI MCP Server")
	common.WithFields(map[string]interface{}{
		"genai_provider":     config.GenAIProvider,
		"genai_base_url":     config.GenAIBaseURL,
		"genai_gen_model":    config.GenAIGenModelName,
		"genai_edit_model":   config.GenAIEditModelName,
		"api_key":            maskAPIKey(config.GenAIAPIKey),
		"server_address":     config.GetServerAddr(),
		"genai_image_format": config.GenAIImageFormat,
	}).Info("Server configuration loaded")

	// 创建 MCP 服务器
	common.Info("Creating MCP server")
	mcpServer := server.NewMCPServer(
		"GenAI MCP Server",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	// 根据 GENAI_PROVIDER 注册对应的工具
	switch config.GenAIProvider {
	case "wan":
		// 初始化 Wan 客户端并注册 Wan tools
		common.Info("Initializing Wan client")
		wanClient, err := wan.NewWanClientFromConfig(config)
		if err != nil {
			common.WithError(err).Fatal("Failed to create Wan client")
		}
		defer wanClient.Close()
		common.Info("Wan client initialized successfully")

		common.Info("Registering Wan tools")
		if err := tools.RegisterWanTools(mcpServer, wanClient); err != nil {
			common.WithError(err).Fatal("Failed to register Wan tools")
		}
		common.Info("Wan tools registered successfully")
	case "apimart":
		// 初始化 APIMart 客户端并注册 APIMart tools
		common.Info("Initializing APIMart client")
		apimartClient, err := apimart.NewApimartClientFromConfig(config)
		if err != nil {
			common.WithError(err).Fatal("Failed to create APIMart client")
		}
		defer apimartClient.Close()
		common.Info("APIMart client initialized successfully")

		common.Info("Registering APIMart tools")
		if err := tools.RegisterApimartTools(mcpServer, apimartClient); err != nil {
			common.WithError(err).Fatal("Failed to register APIMart tools")
		}
		common.Info("APIMart tools registered successfully")
	default:
		// 默认使用 Gemini
		common.Info("Initializing Gemini client")
		geminiClient, err := gemini.NewGeminiClientFromConfig(config)
		if err != nil {
			common.WithError(err).Fatal("Failed to create Gemini client")
		}
		defer geminiClient.Close()
		common.Info("Gemini client initialized successfully")

		common.Info("Registering Gemini tools")
		// 编辑工具的最大图片数与编辑模型相关，因此这里传入编辑模型名称
		if err := tools.RegisterGeminiTools(mcpServer, geminiClient, config.GenAIEditModelName); err != nil {
			common.WithError(err).Fatal("Failed to register Gemini tools")
		}
		common.Info("Gemini tools registered successfully")
	}

	// 创建 Streamable HTTP 服务器
	common.Info("Creating Streamable HTTP server")
	httpServer := server.NewStreamableHTTPServer(
		mcpServer,
		server.WithEndpointPath("/mcp"),
		server.WithHeartbeatInterval(30*time.Second),
	)

	// 设置优雅关闭
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 在 goroutine 中启动服务器
	go func() {
		common.WithField("address", config.GetServerAddr()+"/mcp").Info("MCP server starting")
		if err := httpServer.Start(config.GetServerAddr()); err != nil && err != http.ErrServerClosed {
			common.WithError(err).Fatal("Server error")
		}
	}()

	// 等待中断信号
	<-sigChan
	common.Info("Received shutdown signal, shutting down server...")

	// 优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		common.WithError(err).Fatal("Server shutdown error")
	}

	common.Info("Server stopped gracefully")
}

// maskAPIKey 隐藏 API Key 的敏感部分
func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}
