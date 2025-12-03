package gemini

import "context"

type GenimiIface interface {
	GenerateImage(ctx context.Context, prompt string) (string, error)
	EditImage(ctx context.Context, prompt string, image_url string) (string, error)
}
