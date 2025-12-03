#!/usr/bin/env python3
"""
Test script for image editing using Gemini
"""
import sys
import json
from mcp_client import MCPClient


def main():
    """Test image editing"""
    # Parse command line arguments
    if len(sys.argv) < 3:
        print("Usage: python test_edit_image.py <prompt> <image_url> [base_url]")
        print("Example: python test_edit_image.py 'Make it blue' 'https://yiavatar-images-bj.oss-cn-beijing.aliyuncs.com/images/2025-12-03/0a2017bf-d54e-4858-ae94-6deaf5e16c2a_1764752000_b10b7006.png'")
        return 1
    
    prompt = sys.argv[1]
    image_url = sys.argv[2]
    base_url = sys.argv[3] if len(sys.argv) > 3 else "http://127.0.0.1:8080/mcp"
    
    print(f"Testing image editing with MCP server at: {base_url}")
    print("=" * 60)
    print(f"Prompt: {prompt}")
    print(f"Image URL: {image_url}")
    print("=" * 60)
    
    # Create MCP client
    client = MCPClient(base_url)
    
    # Initialize the session
    print("\n1. Initializing MCP session...")
    init_response = client.initialize(
        client_info={
            "name": "test-client",
            "version": "1.0.0"
        }
    )
    
    if "error" in init_response:
        print(f"❌ Failed to initialize: {init_response['error']}")
        return 1
    
    print("✅ Session initialized successfully")
    
    # Edit image
    print(f"\n2. Editing image with prompt: '{prompt}'...")
    print(f"   Image URL: {image_url}")
    print("   (This may take a while...)")
    
    result = client.edit_image(prompt, image_url)
    
    if "error" in result:
        print(f"❌ Failed to edit image: {result['error']}")
        return 1
    
    if "result" in result:
        result_data = result["result"]
        
        # Check for error in result
        if "isError" in result_data and result_data["isError"]:
            error_content = result_data.get("content", [])
            if error_content:
                error_text = error_content[0].get("text", "Unknown error")
                print(f"❌ Error: {error_text}")
                return 1
        
        # Extract edited image URL or data URI
        content = result_data.get("content", [])
        if content:
            text_content = content[0].get("text", "")
            print("✅ Image edited successfully!")
            print(f"\nResult: {text_content}")
            
            # Check if it's a data URI or URL
            if text_content.startswith("data:"):
                print("\n📸 Edited image is a data URI (base64 encoded)")
                print(f"   Length: {len(text_content)} characters")
                print("   (You can use this in an <img> tag or save it to a file)")
            elif text_content.startswith("http://") or text_content.startswith("https://"):
                print(f"\n📸 Edited image URL: {text_content}")
                print("   (You can open this URL in a browser to view the edited image)")
            else:
                print(f"\n📸 Edited image: {text_content}")
        else:
            print("❌ No content in response")
            print(json.dumps(result, indent=2))
            return 1
    else:
        print("❌ Unexpected response format")
        print(json.dumps(result, indent=2))
        return 1
    
    print("\n" + "=" * 60)
    print("✅ Test completed successfully!")
    return 0


if __name__ == "__main__":
    sys.exit(main())

