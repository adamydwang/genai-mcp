package apimart

import "context"

type ApimartIface interface {
	CreateGenerateImageTask(ctx context.Context, prompt string, size string, resolution string, n int) (string, error)
	QueryGenerateImageTask(ctx context.Context, task_id string) (string, error)
	// CreateEditImageTask 进行图片编辑。
	// - prompt: 编辑文案
	// - image_urls: 输入图片 URL 列表（支持 base64 data URI）
	// - mask_url: 可选的蒙版图片 URL（PNG 格式）
	CreateEditImageTask(ctx context.Context, prompt string, image_urls []string, mask_url string) (string, error)
	QueryEditImageTask(ctx context.Context, task_id string) (string, error)
}
