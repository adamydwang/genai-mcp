<h1>genai-mcp: GenAI MCP Server for Image Generation(eg. Nano Banana)</h1>


## GenAI MCP 服务器

本项目实现了一个 **Model Context Protocol (MCP)** 服务器，用于方便快捷的为用户实现图片生成和图片编辑服务。

- 使用 **Google Gemini / 通义万相等GenAI服务** 进行 **文生图**（generate image）
- 使用 **Google Gemini / 通义万相等主流GenAI服务** 进行 **图像编辑**（edit image）
- 可选：将生成 / 编辑后的图片自动上传到 **S3 兼容对象存储**（如 AWS S3、阿里云 OSS 等），并返回图片 URL

服务器通过 **streamable HTTP** 暴露 MCP 端点。

### 模型类型和第三方提供商支持情况

| 提供商 | 模型名 | 对应配置文件 | 说明 |
|-------|--------|-----------|-----|
| google | · `gemini-3-pro-image-preview` <br>· `gemini-2.5-flash-image` | `GENAI_PROVIDER=gemini` <br> `GENAI_BASE_URL=https://generativelanguage.googleapis.com` | 谷歌官方, 中国用不了 |
| dmxapi | · `gemini-3-pro-image-preview` <br>· `gemini-2.5-flash-image` | `GENAI_PROVIDER=gemini` <br> `GENAI_BASE_URL=https://www.dmxapi.cn` | dmxapi对api的封装完全兼容官方genai sdk |
| aliyun | · `wan2.5-i2i-preview` <br>· `wan2.5-t2i-preview` | `GENAI_PROVIDER=wan` <br> `GENAI_BASE_URL=https://dashscope.aliyuncs.com` | 阿里云官方的通义万相 |
| apimart | · `gemini-3-pro-image-preview` | `GENAI_PROVIDER=apimart` <br> `GENAI_BASE_URL=https://api.apimart.ai` | apimart对接口重新进行了封装，更简洁，<span style="color: red; font-weight: bold;">目前价格最便宜</span> |

---

### 1. 环境依赖

- Go **1.21+**（推荐）
- 有效的 Gemini API Key
- （可选）S3 / OSS 对象存储桶，用于保存图片

---

### 2. 配置（`.env`）

先复制 `env.example` 为 `.env`，然后填写实际值。

**GenAI 配置**

```env
# GenAI 提供方：
# - gemini: Google Gemini / 兼容后端
# - wan:    通义万相（阿里百炼）图片接口
GENAI_PROVIDER=gemini

# 共有的 GenAI 端点和密钥（Gemini / Wan 共用）
GENAI_BASE_URL=https://generativelanguage.googleapis.com
GENAI_API_KEY=your_api_key_here

# 模型名称：
# - 当 GENAI_PROVIDER=gemini 时：如 gemini-3-pro-image-preview
# - 当 GENAI_PROVIDER=wan 时：   如 wan2.5-t2i-preview / wan2.5-i2i-preview
GENAI_GEN_MODEL_NAME=gemini-3-pro-image-preview
GENAI_EDIT_MODEL_NAME=gemini-3-pro-image-preview

# 单次请求超时时间（秒），包括生成图和编辑图
GENAI_TIMEOUT_SECONDS=120

# 图片输出格式：
# - base64: 返回 base64 编码的 data URI
# - url:    上传到 OSS 并返回图片 URL
GENAI_IMAGE_FORMAT=base64
```

**HTTP 服务配置**

```env
SERVER_ADDRESS=0.0.0.0
SERVER_PORT=8080
```

MCP 端点地址：

```text
http://SERVER_ADDRESS:SERVER_PORT/mcp
```

**OSS / S3 配置（当 `GENAI_IMAGE_FORMAT=url` 时必需）**

```env
# 对于 AWS S3：OSS_ENDPOINT 留空或设为 s3.amazonaws.com
# 对于阿里云 OSS：设为 oss-cn-beijing.aliyuncs.com（根据你的 region）
# 对于腾讯 COS：设为 cos.ap-guangzhou.myqcloud.com（根据你的 region）
# 对于 MinIO：设为你的 MinIO 端点
OSS_ENDPOINT=
OSS_REGION=us-east-1
OSS_ACCESS_KEY=your_access_key_here
OSS_SECRET_KEY=your_secret_key_here
OSS_BUCKET=your_bucket_name
```

当 `GENAI_IMAGE_FORMAT=url`：

- 对阿里云 OSS：
  - `OSS_ENDPOINT` 应该是 `oss-<region>.aliyuncs.com` 形式
  - Bucket 策略需要允许你期望的访问方式（例如公开读）

---

### 3. 启动 MCP 服务器

你可以通过 **两种方式** 启动 MCP 服务器：

1. **克隆代码并本地编译运行**
   - 克隆本仓库并进入项目根目录  
   - 将 `env.example` 复制为 `.env`，并填写你的实际配置  
   - 执行：

     ```bash
     go build .
     ./genai-mcp
     ```

2. **下载发布的二进制文件（Release binary）**
   - 从 Releases 页面下载适合你平台的二进制文件  
   - 放到任意目录  
   - 将本仓库（或 Release 附带）的 `env.example` 复制到同一目录并改名为 `.env`，然后修改配置  
   - 执行（文件名视实际发布而定）：

     ```bash
     ./genai-mcp
     ```

默认 MCP HTTP 端点为：

```text
http://127.0.0.1:8080/mcp
```

你可以在任何支持 MCP `streamable-http` 协议的客户端中配置此端点。

---

### 4. MCP 工具说明

定义位置：`internal/tools/gemini.go`

- **`gemini_generate_image`**
  - **输入：**
    - `prompt`（string，必填）：描述要生成图片内容的文本
  - **输出：**
    - `GENAI_IMAGE_FORMAT=base64`：返回 base64 data URI
    - `GENAI_IMAGE_FORMAT=url`：上传到 OSS/S3 后返回图片 URL

- **`gemini_edit_image`**
  - **输入：**
    - `prompt`（string，必填）：描述如何编辑图片
    - `image_url`（string，必填）：原始图片 URL 或 data URI
  - **输出：**
    - 同上，取决于 `GENAI_IMAGE_FORMAT`

当 `GENAI_IMAGE_FORMAT=url` 时：

- 生成 / 编辑后的图片会：
  - 若 Gemini 返回 URL：先下载图片
  - 若返回内联数据：直接使用数据
  - 之后上传到 OSS/S3
  - 路径格式：`images/yyyy-MM-dd/{uuid_timestamp_random}.ext`

---

### 5. 交流方式

- **微信**：请扫码下方二维码添加好友  

  ![微信二维码](assets/wechat_qrcode.png)

- **Discord**：<img src="https://cdn.jsdelivr.net/gh/twitter/twemoji@14.0.2/assets/svg/1f916.svg" alt="discord" width="18" height="18" style="vertical-align:middle;"> [加入社区](https://discord.gg/PHfCTksx)

---

## Star 历史

[![Star History Chart](https://api.star-history.com/svg?repos=adamydwang/genai-mcp&type=Date)](https://star-history.com/#adamydwang/genai-mcp)