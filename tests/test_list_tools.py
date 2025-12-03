#!/usr/bin/env python3
"""
Test script for listing all available MCP tools
"""
import sys
import json
from mcp_client import MCPClient


def main():
    """Test listing all tools"""
    # Parse command line arguments
    base_url = sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:8080/mcp"
    
    print(f"Testing MCP server at: {base_url}")
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
    if "result" in init_response:
        server_info = init_response["result"].get("serverInfo", {})
        print(f"   Server: {server_info.get('name', 'Unknown')} v{server_info.get('version', 'Unknown')}")
    
    # List all tools
    print("\n2. Listing all available tools...")
    tools_response = client.list_tools()
    
    if "error" in tools_response:
        print(f"❌ Failed to list tools: {tools_response['error']}")
        return 1
    
    if "result" in tools_response:
        tools = tools_response["result"].get("tools", [])
        print(f"✅ Found {len(tools)} tool(s):\n")
        
        for i, tool in enumerate(tools, 1):
            print(f"Tool {i}: {tool.get('name', 'Unknown')}")
            print(f"  Description: {tool.get('description', 'No description')}")
            
            # Print input schema if available
            input_schema = tool.get("inputSchema", {})
            if input_schema:
                properties = input_schema.get("properties", {})
                required = input_schema.get("required", [])
                
                if properties:
                    print("  Parameters:")
                    for param_name, param_info in properties.items():
                        param_type = param_info.get("type", "unknown")
                        param_desc = param_info.get("description", "")
                        required_mark = " (required)" if param_name in required else ""
                        print(f"    - {param_name}: {param_type}{required_mark}")
                        if param_desc:
                            print(f"      {param_desc}")
            
            print()
    else:
        print("❌ Unexpected response format")
        print(json.dumps(tools_response, indent=2))
        return 1
    
    print("=" * 60)
    print("✅ Test completed successfully!")
    return 0


if __name__ == "__main__":
    sys.exit(main())

