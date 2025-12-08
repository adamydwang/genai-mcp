package common

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config 应用配置结构
type Config struct {
	// GenAI 提供方: gemini、wan 或 apimart
	GenAIProvider string

	// 通用 GenAI 配置（Gemini 和 Wan 共用同一套 BaseURL / APIKey）
	GenAIBaseURL string
	GenAIAPIKey  string
	// 分别用于图片生成与图片编辑的模型名称
	GenAIGenModelName  string
	GenAIEditModelName string

	ServerAddress string
	ServerPort    string
	// OSS 配置
	OSSEndpoint  string
	OSSRegion    string
	OSSAccessKey string
	OSSSecretKey string
	OSSBucket    string
	// 图片输出格式: base64 或 url
	GenAIImageFormat string
	// GenAI 请求超时时间（秒）
	GenAITimeoutSeconds int
	// 日志配置
	LogLevel  string // 日志级别: debug, info, warn, error
	LogFormat string // 日志格式: json, text
	LogOutput string // 输出位置: stdout, stderr, file
	LogFile   string // 日志文件路径（当 LogOutput 为 file 时）
}

// LoadConfig 从 .env 文件加载配置
func LoadConfig() (*Config, error) {
	// 加载 .env 文件（如果存在）
	if err := godotenv.Load(); err != nil {
		// .env 文件不存在时，尝试从环境变量读取
		fmt.Println("Warning: .env file not found, using environment variables")
	}

	config := &Config{
		GenAIProvider:      getEnv("GENAI_PROVIDER", "gemini"),
		GenAIBaseURL:       getEnv("GENAI_BASE_URL", ""),
		GenAIAPIKey:        getEnv("GENAI_API_KEY", ""),
		GenAIGenModelName:  getEnv("GENAI_GEN_MODEL_NAME", ""),
		GenAIEditModelName: getEnv("GENAI_EDIT_MODEL_NAME", ""),
		ServerAddress:      getEnv("SERVER_ADDRESS", "0.0.0.0"),
		ServerPort:         getEnv("SERVER_PORT", "8080"),
		// OSS 配置
		OSSEndpoint:         getEnv("OSS_ENDPOINT", ""),
		OSSRegion:           getEnv("OSS_REGION", "us-east-1"),
		OSSAccessKey:        getEnv("OSS_ACCESS_KEY", ""),
		OSSSecretKey:        getEnv("OSS_SECRET_KEY", ""),
		OSSBucket:           getEnv("OSS_BUCKET", ""),
		GenAIImageFormat:    getEnv("GENAI_IMAGE_FORMAT", "base64"),
		GenAITimeoutSeconds: getEnvInt("GENAI_TIMEOUT_SECONDS", 60),
		// 日志配置
		LogLevel:  getEnv("LOG_LEVEL", "info"),
		LogFormat: getEnv("LOG_FORMAT", "text"),
		LogOutput: getEnv("LOG_OUTPUT", "stdout"),
		LogFile:   getEnv("LOG_FILE", ""),
	}

	// 根据提供方校验必需的配置（Gemini、Wan 和 APIMart 共用 GENAI_* 三个字段）
	switch config.GenAIProvider {
	case "wan", "gemini", "apimart":
		if config.GenAIAPIKey == "" {
			return nil, fmt.Errorf("GENAI_API_KEY is required when GENAI_PROVIDER=%s", config.GenAIProvider)
		}
	default:
		return nil, fmt.Errorf("unsupported GENAI_PROVIDER: %s", config.GenAIProvider)
	}

	// 初始化日志系统
	logConfig := &LogConfig{
		Level:    config.LogLevel,
		Format:   config.LogFormat,
		Output:   config.LogOutput,
		FilePath: config.LogFile,
	}
	if err := InitLogger(logConfig); err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	return config, nil
}

// getEnv 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvBool 获取布尔类型环境变量
func getEnvBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value == "true" || value == "1" || value == "yes" || value == "on"
}

// getEnvInt 获取整型环境变量
func getEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	if i, err := strconv.Atoi(value); err == nil {
		return i
	}
	return defaultValue
}

// GetServerAddr 返回完整的服务器地址
func (c *Config) GetServerAddr() string {
	return fmt.Sprintf("%s:%s", c.ServerAddress, c.ServerPort)
}

// GetOSSConfig 返回 OSS 配置，用于创建 OSS 客户端
func (c *Config) GetOSSConfig() map[string]string {
	return map[string]string{
		"endpoint":  c.OSSEndpoint,
		"region":    c.OSSRegion,
		"accessKey": c.OSSAccessKey,
		"secretKey": c.OSSSecretKey,
		"bucket":    c.OSSBucket,
	}
}
