# MCP Server Test Scripts

This directory contains Python test scripts for testing the GenAI MCP Server.

## Prerequisites

1. Python 3.7 or higher
2. Install dependencies:
   ```bash
   pip install -r requirements.txt
   ```

## Setup

1. Make sure the MCP server is running:
   ```bash
   # In the project root directory
   go run server.go
   ```

2. The server should be listening on `http://127.0.0.1:8080/mcp` by default.

## Test Scripts

### 1. test_list_tools.py

Lists all available tools from the MCP server.

**Usage:**
```bash
python test_list_tools.py [base_url]
```

**Example:**
```bash
python test_list_tools.py
python test_list_tools.py http://127.0.0.1:8080/mcp
```

**Output:**
- Lists all available tools
- Shows tool descriptions
- Displays tool parameters and their types

### 2. test_generate_image.py

Tests the image generation functionality. Supports multiple providers:
- **gemini**: Synchronous image generation
- **wan**: Asynchronous image generation (Ali Bailian Wanxiang)
- **apimart**: Asynchronous image generation (APIMart)

**Usage:**
```bash
python test_generate_image.py <prompt> [base_url]
```

**Example:**
```bash
python test_generate_image.py "A beautiful sunset over mountains"
python test_generate_image.py "A cute cat playing with a ball" http://127.0.0.1:8080/mcp
```

**Note:** The provider is determined by the `GENAI_PROVIDER` environment variable (default: "gemini").

**Output:**
- Generates an image based on the prompt
- Returns the image URL or data URI
- Displays the result

### 3. test_edit_image.py

Tests the image editing functionality. Supports multiple providers:
- **gemini**: Synchronous image editing
- **wan**: Asynchronous image editing (Ali Bailian Wanxiang) - supports single image URL only
- **apimart**: Asynchronous image editing (APIMart) - supports multiple images and base64 data URIs

**Usage:**
```bash
python test_edit_image.py <prompt> <image_url> [base_url]
```

**Example:**
```bash
python test_edit_image.py "Make it blue" "https://example.com/image.jpg"
python test_edit_image.py "Add a rainbow" "data:image/png;base64,..." http://127.0.0.1:8080/mcp
```

**Output:**
- Edits the image based on the prompt
- Returns the edited image URL or data URI
- Displays the result

## Running All Tests

### Option 1: Run All Tests Automatically

Use the `run_all_tests.py` script to run all tests in sequence:

```bash
python run_all_tests.py [base_url]
```

**Example:**
```bash
python run_all_tests.py
python run_all_tests.py http://127.0.0.1:8080/mcp
```

### Option 2: Run Tests Individually

You can also run each test script separately:

```bash
# 1. List tools
python test_list_tools.py

# 2. Generate an image
python test_generate_image.py "A beautiful landscape"

# 3. Edit an image (using the generated image URL from step 2)
python test_edit_image.py "Make it more colorful" "<image_url_from_step_2>"
```

## Troubleshooting

1. **Connection Error**: Make sure the MCP server is running and accessible at the specified URL.

2. **Timeout Error**: Image generation/editing may take some time. The default timeout is 60 seconds. You can modify it in `mcp_client.py` if needed.

3. **Authentication Error**: Make sure the server has valid API keys configured in the `.env` file.

4. **Tool Not Found**: Ensure the server has registered the tools correctly. Check the server logs for any errors.

## Provider Support

The test scripts support multiple GenAI providers, determined by the `GENAI_PROVIDER` environment variable:

- **gemini** (default): Google Gemini API
  - Synchronous operations
  - Supports multiple images for editing
  
- **wan**: Ali Bailian Wanxiang API
  - Asynchronous operations
  - Single image URL only (no base64/data URIs)
  
- **apimart**: APIMart API
  - Asynchronous operations
  - Supports multiple images and base64 data URIs
  - Supports optional mask images for editing

Set the provider before running tests:
```bash
export GENAI_PROVIDER=apimart
python test_generate_image.py "A beautiful landscape"
```

## Notes

- All test scripts are independent and can be run separately
- The scripts use JSON-RPC 2.0 protocol to communicate with the MCP server
- Results may include data URIs (base64 encoded images) or URLs depending on server configuration
- If OSS upload is enabled, the server will upload images to OSS and return signed URLs
- For asynchronous providers (wan, apimart), the scripts automatically poll for task completion

