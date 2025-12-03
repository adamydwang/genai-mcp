package main

import (
	"fmt"
	"log"
	"os"

	"genai-mcp/common"
	"genai-mcp/internal/genai/gemini"
	"genai-mcp/internal/tools"

	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// 加载配置
	config, err := common.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 打印配置信息（隐藏敏感信息）
	fmt.Fprintf(os.Stderr, "Server starting...\n")
	fmt.Fprintf(os.Stderr, "GenAI Base URL: %s\n", config.GenAIBaseURL)
	fmt.Fprintf(os.Stderr, "GenAI Model: %s\n", config.GenAIModelName)
	fmt.Fprintf(os.Stderr, "API Key: %s\n", maskAPIKey(config.GenAIAPIKey))

	// 创建 Gemini 客户端
	geminiClient, err := gemini.NewGeminiClientFromConfig(config)
	if err != nil {
		log.Fatalf("Failed to create Gemini client: %v", err)
	}
	defer geminiClient.Close()

	// 创建 MCP 服务器
	s := server.NewMCPServer(
		"GenAI MCP Server",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	// 注册 Gemini tools
	if err := tools.RegisterGeminiTools(s, geminiClient); err != nil {
		log.Fatalf("Failed to register Gemini tools: %v", err)
	}

	// 启动 stdio 服务器
	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// maskAPIKey 隐藏 API Key 的敏感部分
func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}
