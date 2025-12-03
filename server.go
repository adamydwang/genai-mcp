package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"genai-mcp/common"
	"genai-mcp/internal/genai/gemini"
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
		"genai_base_url":     config.GenAIBaseURL,
		"genai_model":        config.GenAIModelName,
		"api_key":            maskAPIKey(config.GenAIAPIKey),
		"server_address":     config.GetServerAddr(),
		"genai_image_format": config.GenAIImageFormat,
	}).Info("Server configuration loaded")

	// 创建 Gemini 客户端
	common.Info("Initializing Gemini client")
	geminiClient, err := gemini.NewGeminiClientFromConfig(config)
	if err != nil {
		common.WithError(err).Fatal("Failed to create Gemini client")
	}
	defer geminiClient.Close()
	common.Info("Gemini client initialized successfully")

	// 创建 MCP 服务器
	common.Info("Creating MCP server")
	mcpServer := server.NewMCPServer(
		"GenAI MCP Server",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	// 注册 Gemini tools
	common.Info("Registering Gemini tools")
	if err := tools.RegisterGeminiTools(mcpServer, geminiClient); err != nil {
		common.WithError(err).Fatal("Failed to register Gemini tools")
	}
	common.Info("Gemini tools registered successfully")

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
