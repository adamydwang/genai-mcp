package oss

import (
	"genai-mcp/common"
)

// NewOSSClientFromConfig 从配置创建 OSS 客户端
func NewOSSClientFromConfig(cfg *common.Config) (OSSIface, error) {
	ossCfg := S3Config{
		Endpoint:  cfg.OSSEndpoint,
		Region:    cfg.OSSRegion,
		AccessKey: cfg.OSSAccessKey,
		SecretKey: cfg.OSSSecretKey,
	}

	return NewS3Client(ossCfg)
}
