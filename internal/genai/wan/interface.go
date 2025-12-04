package wan

import "context"

type WanIface interface {
	CreateGenerateImageTask(ctx context.Context, prompt string, negative_prompt string) (string, error)
	QueryGenerateImageTask(ctx context.Context, task_id string) (string, error)
	// CreateEditImageTask 进行图片编辑 / 融合。
	// - prompt: 编辑/融合文案
	// - image_urls: 输入图片 URL 列表（单图编辑或多图融合）
	CreateEditImageTask(ctx context.Context, prompt string, image_urls []string) (string, error)
	QueryEditImageTask(ctx context.Context, task_id string) (string, error)
}
