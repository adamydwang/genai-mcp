<h1>genai-mcp: GenAI MCP Server for Image Generation (e.g. Nano Banana)</h1>

## GenAI MCP Server

This project implements a **Model Context Protocol (MCP) server** for image generation and image editing using **Google Gemini**, **Tongyi Wanxiang**, plus optional automatic upload of generated images to **S3‑compatible object storage** (AWS S3, Aliyun OSS, etc.).

The server exposes a **streamable HTTP MCP endpoint** and provides tools for Gemini, Wan, and APIMart.

### Provider matrix

| Provider | Supported Models | Config | Notes |
| --- | --- | --- | --- |
| google | · `gemini-3-pro-image-preview` <br> · `gemini-2.5-flash-image` | `GENAI_PROVIDER=gemini`<br>`GENAI_BASE_URL=https://generativelanguage.googleapis.com` | Official Gemini |
| dmxapi | · `gemini-3-pro-image-preview` <br> · `gemini-2.5-flash-image` | `GENAI_PROVIDER=gemini`<br>`GENAI_BASE_URL=https://www.dmxapi.cn` | Gemini‑compatible gateway |
| aliyun | · `wan2.5-i2i-preview` <br> · `wan2.5-t2i-preview` | `GENAI_PROVIDER=wan`<br>`GENAI_BASE_URL=https://dashscope.aliyuncs.com` | Tongyi Wanxiang |
| apimart | · `gemini-3-pro-image-preview` | `GENAI_PROVIDER=apimart`<br>`GENAI_BASE_URL=https://api.apimart.ai` | APIMart Gemini wrapper (cost‑effective) |

---

### 1. Prerequisites

- Go **1.21+** (recommended)
- A valid Gemini (or provider) API key
- Optional: S3 / OSS bucket for storing images

---

### 2. Configuration (`.env`)

Copy `env.example` to `.env`, then fill in real values.

**GenAI configuration**

```env
# GenAI provider:
# - gemini: Google Gemini / compatible backend
# - wan:    Ali Bailian Tongyi Wanxiang image APIs
# - apimart: APIMart (Gemini-wrapped async image APIs)
GENAI_PROVIDER=gemini

# Shared GenAI endpoint / key for all providers
GENAI_BASE_URL=https://generativelanguage.googleapis.com
GENAI_API_KEY=your_api_key_here

# Model names:
# - When GENAI_PROVIDER=gemini: Gemini model names, e.g. gemini-3-pro-image-preview
# - When GENAI_PROVIDER=wan:    Wanxiang model names, e.g. wan2.5-t2i-preview / wan2.5-i2i-preview
# - When GENAI_PROVIDER=apimart: Gemini image model name, e.g. gemini-3-pro-image-preview
GENAI_GEN_MODEL_NAME=gemini-3-pro-image-preview
GENAI_EDIT_MODEL_NAME=gemini-3-pro-image-preview

# Request timeout in seconds for each GenAI call (generate / edit)
GENAI_TIMEOUT_SECONDS=120

# Image output format:
# - base64: return image as data URI (base64 encoded)
# - url:    upload image to OSS and return plain URL
GENAI_IMAGE_FORMAT=base64
```

**HTTP server**

```env
SERVER_ADDRESS=0.0.0.0
SERVER_PORT=8080
```

MCP endpoint:

```text
http://SERVER_ADDRESS:SERVER_PORT/mcp
```

**OSS / S3 configuration (optional, required when `GENAI_IMAGE_FORMAT=url`)**

```env
# For AWS S3: leave OSS_ENDPOINT empty or set to s3.amazonaws.com
# For Aliyun OSS: set to oss-cn-hangzhou.aliyuncs.com or your region
# For Tencent COS: set to cos.ap-guangzhou.myqcloud.com
# For MinIO: set to your MinIO endpoint
OSS_ENDPOINT=
OSS_REGION=us-east-1
OSS_ACCESS_KEY=your_access_key_here
OSS_SECRET_KEY=your_secret_key_here
OSS_BUCKET=your_bucket_name
```

When `GENAI_IMAGE_FORMAT=url`:

- For **Aliyun OSS**: ensure `OSS_ENDPOINT` like `oss-cn-beijing.aliyuncs.com` and bucket policy allows expected read access.

---

### 3. Running the MCP Server

You can run the MCP server in **two ways**:

1. **Clone & build from source**
   - Clone this repo and enter the project root  
   - Copy `env.example` to `.env` and fill in your configuration  
   - Run:

     ```bash
     go build .
     ./genai-mcp
     ```

2. **Download release binary**
   - Download the appropriate binary from the Releases page  
   - Place it in a directory of your choice  
   - Copy `env.example` (from repo or release asset) to `.env` and update configuration  
   - Run (binary name may vary by platform):

     ```bash
     ./genai-mcp
     ```

Default MCP HTTP endpoint:

```text
http://127.0.0.1:8080/mcp
```

Connect from any MCP‑compatible client supporting streamable HTTP transport.

---

### 4. MCP Tools

#### Gemini tools (`internal/tools/gemini.go`)

- **`gemini_generate_image`**
  - **Input**: `prompt` (string, required)
  - **Output**: base64 data URI or URL (optional OSS upload)

- **`gemini_edit_image`**
  - **Input**: `prompt` (required), `image_urls` (required, JSON array)
  - **Output**: base64 data URI or URL

When `GENAI_IMAGE_FORMAT=url`, images are downloaded/decoded then uploaded to OSS/S3 under `images/yyyy-MM-dd/{uuid_timestamp_random}.ext`.

#### Wan tools (`internal/tools/wan.go`)

- `wan_create_generate_image_task`
- `wan_query_generate_image_task`
- `wan_create_edit_image_task`
- `wan_query_edit_image_task`

Wan is async; create a task then poll for completion.

#### APIMart tools (`internal/tools/apimart.go`)

- `apimart_create_generate_image_task`
- `apimart_query_generate_image_task`
- `apimart_create_edit_image_task`
- `apimart_query_edit_image_task`

APIMart is async; tools return the final image (URL or base64) once the task is completed.

---

### 5. Contact

- **WeChat**: Scan the QR code below to add as a friend  

  ![WeChat QR Code](assets/wechat_qrcode.png)

- **Discord**: <img src="https://cdn.jsdelivr.net/gh/twitter/twemoji@14.0.2/assets/svg/1f916.svg" alt="discord" width="18" height="18" style="vertical-align:middle;"> [Join the community](https://discord.gg/PHfCTksx)

---

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=adamydwang/genai-mcp&type=Date)](https://star-history.com/#adamydwang/genai-mcp)
