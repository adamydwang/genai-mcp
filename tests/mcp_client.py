"""
MCP Client utility for testing MCP server
"""
import json
import uuid
from typing import Dict, Any, Optional
import requests


class MCPClient:
    """MCP Streamable HTTP Client"""
    
    def __init__(self, base_url: str = "http://127.0.0.1:8080/mcp"):
        """
        Initialize MCP client
        
        Args:
            base_url: Base URL of the MCP server
        """
        self.base_url = base_url.rstrip('/')
        self.session_id: Optional[str] = None
        
    def _generate_request_id(self) -> str:
        """Generate a unique request ID"""
        return str(uuid.uuid4())
    
    def _send_request(self, method: str, params: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
        """
        Send a JSON-RPC request to the MCP server
        
        Args:
            method: JSON-RPC method name
            params: Method parameters
            
        Returns:
            Response from the server
        """
        request = {
            "jsonrpc": "2.0",
            "id": self._generate_request_id(),
            "method": method,
        }
        
        if params:
            request["params"] = params
            
        headers = {
            "Content-Type": "application/json",
        }
        
        # Use Mcp-Session-Id header for session management
        if self.session_id:
            headers["Mcp-Session-Id"] = self.session_id
        
        try:
            # Disable proxy for localhost connections to avoid 502 errors
            proxies = {
                'http': None,
                'https': None,
            }
            
            response = requests.post(
                self.base_url,
                json=request,
                headers=headers,
                timeout=600,
                proxies=proxies
            )
            response.raise_for_status()
            
            # Extract session ID from response headers if available
            # Always update session ID if present in response (may change)
            session_id = response.headers.get('Mcp-Session-Id') or response.headers.get('X-Session-Id')
            if session_id:
                self.session_id = session_id
            
            return response.json()
        except requests.exceptions.HTTPError as e:
            # Get more details about HTTP errors
            error_msg = f"HTTP {e.response.status_code}"
            if e.response.text:
                error_msg += f": {e.response.text[:200]}"
            return {
                "error": {
                    "code": -32603,
                    "message": error_msg
                }
            }
        except requests.exceptions.RequestException as e:
            return {
                "error": {
                    "code": -32603,
                    "message": f"Request failed: {str(e)}"
                }
            }
    
    def initialize(self, protocol_version: str = "2024-11-05", 
                   capabilities: Optional[Dict[str, Any]] = None,
                   client_info: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
        """
        Initialize the MCP session
        
        Args:
            protocol_version: MCP protocol version
            capabilities: Client capabilities
            client_info: Client information
            
        Returns:
            Server response with server info and capabilities
        """
        params = {
            "protocolVersion": protocol_version,
        }
        
        if capabilities:
            params["capabilities"] = capabilities
            
        if client_info:
            params["clientInfo"] = client_info
        
        response = self._send_request("initialize", params)
        
        # Extract session ID from response if available
        if "result" in response and "serverInfo" in response["result"]:
            # Session ID might be in headers or response
            pass
            
        return response
    
    def list_tools(self) -> Dict[str, Any]:
        """
        List all available tools
        
        Returns:
            List of tools from the server
        """
        return self._send_request("tools/list")
    
    def call_tool(self, name: str, arguments: Dict[str, Any]) -> Dict[str, Any]:
        """
        Call a tool
        
        Args:
            name: Tool name
            arguments: Tool arguments
            
        Returns:
            Tool execution result
        """
        params = {
            "name": name,
            "arguments": arguments
        }
        return self._send_request("tools/call", params)
    
    def generate_image(self, prompt: str) -> Dict[str, Any]:
        """
        Generate an image using Gemini
        
        Args:
            prompt: Text prompt describing the image to generate
            
        Returns:
            Generated image URL or data URI
        """
        return self.call_tool("gemini_generate_image", {"prompt": prompt})
    
    def edit_image(self, prompt: str, image_urls: list) -> Dict[str, Any]:
        """
        Edit images using Gemini
        
        Args:
            prompt: Text prompt describing how to edit the image
            image_urls: List of URLs or data URIs of the images to edit
            
        Returns:
            Edited image URL or data URI
        """
        # Convert list to JSON string as required by the MCP tool
        image_urls_json = json.dumps(image_urls)
        return self.call_tool("gemini_edit_image", {
            "prompt": prompt,
            "image_urls": image_urls_json
        })

