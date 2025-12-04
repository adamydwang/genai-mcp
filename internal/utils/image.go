package utils

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DownloadImageFromURL 从 URL 下载图片，返回图片数据和 MIME 类型
func DownloadImageFromURL(ctx context.Context, url string) ([]byte, string, error) {
	// 创建 HTTP 客户端
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("failed to download image: status code %d", resp.StatusCode)
	}

	// 读取图片数据
	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	// 获取 Content-Type
	mimeType := resp.Header.Get("Content-Type")
	if mimeType == "" {
		// 根据文件扩展名推断 MIME 类型
		mimeType = InferMimeTypeFromURL(url)
	}

	return imageData, mimeType, nil
}

// InferMimeTypeFromURL 从 URL 推断 MIME 类型（不区分大小写）
func InferMimeTypeFromURL(url string) string {
	// 简单的 MIME 类型推断
	if len(url) > 4 {
		ext := strings.ToLower(url[len(url)-4:])
		switch ext {
		case ".jpg", "jpeg":
			return "image/jpeg"
		case ".png":
			return "image/png"
		case ".gif":
			return "image/gif"
		case ".webp":
			return "image/webp"
		}
	}
	// 默认返回 jpeg
	return "image/jpeg"
}

// GenerateImagePath 生成图片路径：images/yyyy-MM-dd/
func GenerateImagePath() string {
	now := time.Now()
	// 格式：yyyy-MM-dd
	return fmt.Sprintf("images/%s/", now.Format("2006-01-02"))
}

// GenerateImageFileName 生成图片文件名：{uuid_timestamp_random}
func GenerateImageFileName(mimeType string) string {
	// 生成 UUID（base64 去掉填充，缩短长度）
	uuidBytes := make([]byte, 16)
	_, _ = rand.Read(uuidBytes)
	uuidStr := base64.RawURLEncoding.EncodeToString(uuidBytes)

	// 生成时间戳
	timestamp := time.Now().Unix()

	// 生成随机字符串
	randomBytes := make([]byte, 4)
	_, _ = rand.Read(randomBytes)
	randomStr := fmt.Sprintf("%x", randomBytes)

	// 根据 MIME 类型确定文件扩展名
	ext := GetExtensionFromMimeType(mimeType)

	// 组合文件名：{uuid}_{timestamp}_{random}.ext
	return fmt.Sprintf("%s_%d_%s%s", uuidStr, timestamp, randomStr, ext)
}

// GetExtensionFromMimeType 根据 MIME 类型获取文件扩展名（不区分大小写）
func GetExtensionFromMimeType(mimeType string) string {
	mt := strings.ToLower(mimeType)
	switch mt {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/bmp":
		return ".bmp"
	default:
		return ".jpg" // 默认使用 jpg
	}
}

// TruncateForLog 截断长字符串用于日志，避免打印过长内容（如 base64）
func TruncateForLog(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}
