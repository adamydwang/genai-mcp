package common

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
)

var (
	// Logger 全局日志实例
	Logger *logrus.Logger
)

// LogConfig 日志配置
type LogConfig struct {
	Level      string // 日志级别: debug, info, warn, error
	Format     string // 日志格式: json, text
	Output     string // 输出位置: stdout, stderr, file
	FilePath   string // 日志文件路径（当 Output 为 file 时）
	MaxSize    int    // 日志文件最大大小（MB）
	MaxBackups int    // 保留的旧日志文件数量
	MaxAge     int    // 保留日志文件的天数
	Compress   bool   // 是否压缩旧日志文件
}

// InitLogger 初始化日志系统
func InitLogger(cfg *LogConfig) error {
	logger := logrus.New()

	// 开启调用方信息（文件名和行号）
	logger.SetReportCaller(true)

	// 设置日志级别
	level, err := logrus.ParseLevel(cfg.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)

	// 设置日志格式（包含文件名和行号）
	logger.SetFormatter(newFormatter(cfg.Format))

	// 设置输出
	var output io.Writer
	switch strings.ToLower(cfg.Output) {
	case "stderr":
		output = os.Stderr
	case "file":
		if cfg.FilePath == "" {
			output = os.Stdout
		} else {
			// 确保日志目录存在
			dir := filepath.Dir(cfg.FilePath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return err
			}

			// 打开日志文件
			file, err := os.OpenFile(cfg.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
			if err != nil {
				return err
			}
			output = file
		}
	default:
		output = os.Stdout
	}
	logger.SetOutput(output)

	Logger = logger
	return nil
}

// GetLogger 获取日志实例
func GetLogger() *logrus.Logger {
	if Logger == nil {
		// 如果没有初始化，使用默认配置
		logger := logrus.New()
		logger.SetReportCaller(true)
		logger.SetLevel(logrus.InfoLevel)
		logger.SetFormatter(newFormatter("text"))
		Logger = logger
	}
	return Logger
}

// newFormatter 创建带有文件名和行号信息的 Formatter
func newFormatter(format string) logrus.Formatter {
	// 统一的调用方美化函数，只输出 "filename.go:line"
	callerPretty := func(frame *runtime.Frame) (function string, file string) {
		filename := filepath.Base(frame.File)
		return "", fmt.Sprintf("%s:%d", filename, frame.Line)
	}

	switch strings.ToLower(format) {
	case "json":
		return &logrus.JSONFormatter{
			TimestampFormat:  "2006-01-02 15:04:05",
			CallerPrettyfier: callerPretty,
		}
	default:
		return &logrus.TextFormatter{
			FullTimestamp:    true,
			TimestampFormat:  "2006-01-02 15:04:05",
			CallerPrettyfier: callerPretty,
		}
	}
}

// Debug 记录 Debug 级别日志
func Debug(args ...interface{}) {
	GetLogger().Debug(args...)
}

// Debugf 记录 Debug 级别日志（格式化）
func Debugf(format string, args ...interface{}) {
	GetLogger().Debugf(format, args...)
}

// Info 记录 Info 级别日志
func Info(args ...interface{}) {
	GetLogger().Info(args...)
}

// Infof 记录 Info 级别日志（格式化）
func Infof(format string, args ...interface{}) {
	GetLogger().Infof(format, args...)
}

// Warn 记录 Warn 级别日志
func Warn(args ...interface{}) {
	GetLogger().Warn(args...)
}

// Warnf 记录 Warn 级别日志（格式化）
func Warnf(format string, args ...interface{}) {
	GetLogger().Warnf(format, args...)
}

// Error 记录 Error 级别日志
func Error(args ...interface{}) {
	GetLogger().Error(args...)
}

// Errorf 记录 Error 级别日志（格式化）
func Errorf(format string, args ...interface{}) {
	GetLogger().Errorf(format, args...)
}

// Fatal 记录 Fatal 级别日志并退出
func Fatal(args ...interface{}) {
	GetLogger().Fatal(args...)
}

// Fatalf 记录 Fatal 级别日志并退出（格式化）
func Fatalf(format string, args ...interface{}) {
	GetLogger().Fatalf(format, args...)
}

// WithField 添加字段到日志
func WithField(key string, value interface{}) *logrus.Entry {
	return GetLogger().WithField(key, value)
}

// WithFields 添加多个字段到日志
func WithFields(fields map[string]interface{}) *logrus.Entry {
	return GetLogger().WithFields(logrus.Fields(fields))
}

// WithError 添加错误到日志
func WithError(err error) *logrus.Entry {
	return GetLogger().WithError(err)
}
