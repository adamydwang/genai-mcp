#!/usr/bin/env python3
"""
Test script for image generation using Gemini
"""
import sys
import json
from mcp_client import MCPClient


def main():
    """Test image generation"""
    # Parse command line arguments
    if len(sys.argv) < 2:
        print("Usage: python test_generate_image.py <prompt> [base_url]")
        print("Example: python test_generate_image.py 'A beautiful sunset over mountains'")
        return 1
    
    prompt = sys.argv[1]
    base_url = sys.argv[2] if len(sys.argv) > 2 else "http://127.0.0.1:8080/mcp"
    
    print(f"Testing image generation with MCP server at: {base_url}")
    print("=" * 60)
    print(f"Prompt: {prompt}")
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
    
    # Generate image
    print(f"\n2. Generating image with prompt: '{prompt}'...")
    print("   (This may take a while...)")
    
    result = client.generate_image(prompt)
    
    if "error" in result:
        print(f"❌ Failed to generate image: {result['error']}")
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
        
        # Extract image URL or data URI
        content = result_data.get("content", [])
        if content:
            text_content = content[0].get("text", "")
            print("✅ Image generated successfully!")
            print(f"\nResult: {text_content}")
            
            # Check if it's a data URI or URL
            if text_content.startswith("data:"):
                print("\n📸 Generated image is a data URI (base64 encoded)")
                print(f"   Length: {len(text_content)} characters")
                print("   (You can use this in an <img> tag or save it to a file)")
            elif text_content.startswith("http://") or text_content.startswith("https://"):
                print(f"\n📸 Generated image URL: {text_content}")
                print("   (You can open this URL in a browser to view the image)")
            else:
                print(f"\n📸 Generated image: {text_content}")
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

