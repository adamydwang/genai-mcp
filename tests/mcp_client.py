"""
MCP Client utility for testing MCP server
"""
import json
import os
import time
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
        # Use the same provider as the Go server (GENAI_PROVIDER), default to gemini
        self.provider: str = os.getenv("GENAI_PROVIDER", "gemini").lower()
        
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

    # ---------- Wan helper methods ----------

    def _extract_text_from_mcp_result(self, response: Dict[str, Any]) -> Optional[str]:
        """
        Extract the first text content from an MCP tools/call response.
        Returns None if not found.
        """
        try:
            result = response.get("result", {})
            content = result.get("content", [])
            if not content:
                return None
            return content[0].get("text")
        except Exception:
            return None

    def _parse_wan_task_status(self, raw_json_text: str) -> Optional[str]:
        """
        Parse Wan async task status from the raw JSON string returned by Wan APIs.

        Common patterns (subject to actual Bailian docs):
          - {"output": {"task_status": "SUCCEEDED", ...}}
          - {"output": {"status": "SUCCEEDED", ...}}
          - {"task_status": "SUCCEEDED", ...}

        Returns:
            status string (e.g. "SUCCEEDED", "FAILED", "RUNNING") or None if not found.
        """
        try:
            data = json.loads(raw_json_text)
        except Exception:
            return None

        # Try a few common nesting patterns
        for key in ("output", "result", "data"):
            if isinstance(data, dict) and key in data and isinstance(data[key], dict):
                candidate = data[key]
                status = (
                    candidate.get("task_status")
                    or candidate.get("status")
                    or candidate.get("state")
                )
                if isinstance(status, str):
                    return status

        # Fall back to top-level fields
        if isinstance(data, dict):
            status = (
                data.get("task_status")
                or data.get("status")
                or data.get("state")
            )
            if isinstance(status, str):
                return status

        return None

    def _poll_wan_task(
        self,
        query_tool: str,
        task_id: str,
        *,
        max_attempts: int = 30,
        interval_seconds: float = 2.0,
    ) -> Dict[str, Any]:
        """
        Poll a Wan async task until it reaches a terminal state or times out.

        Args:
            query_tool: MCP tool name for querying (e.g. "wan_query_generate_image_task")
            task_id: Wan task ID
            max_attempts: maximum number of query attempts
            interval_seconds: interval between attempts in seconds

        Returns:
            The last MCP response from the query tool.
        """
        terminal_success = {"succeeded", "success", "finished", "done", "completed"}
        terminal_failed = {"failed", "error", "canceled", "cancelled"}

        last_response: Optional[Dict[str, Any]] = None

        for _ in range(max_attempts):
            resp = self.call_tool(query_tool, {"task_id": task_id})
            last_response = resp

            # If MCP-level error, return immediately
            if "error" in resp:
                return resp

            text = self._extract_text_from_mcp_result(resp)
            if not text:
                # No text content; wait and retry
                time.sleep(interval_seconds)
                continue

            status = self._parse_wan_task_status(text)
            if not status:
                # Cannot determine status, keep polling
                time.sleep(interval_seconds)
                continue

            normalized = status.lower()
            if normalized in terminal_success or any(
                normalized.startswith(s) for s in terminal_success
            ):
                # Task finished successfully
                return resp
            if normalized in terminal_failed or any(
                normalized.startswith(s) for s in terminal_failed
            ):
                # Task failed / canceled
                return resp

            # Otherwise assume still running
            time.sleep(interval_seconds)

        # Reached max attempts; return the last response we got
        return last_response or {
            "error": {
                "code": -32603,
                "message": f"Wan task polling exceeded max attempts ({max_attempts})",
            }
        }
    
    def generate_image(self, prompt: str) -> Dict[str, Any]:
        """
        Generate an image using the configured provider.
        
        For provider == "gemini":
            - Calls gemini_generate_image and returns its response (image URL or data URI).
        
        For provider == "wan":
            - Calls wan_create_generate_image_task to create an async task.
            - Extracts task_id from the text result.
            - Calls wan_query_generate_image_task once with the task_id and returns that response.
              (The actual image URL is contained in the raw JSON text returned by the Wan API.)
        """
        if self.provider == "wan":
            # 1) Create generate-image task
            create_resp = self.call_tool(
                "wan_create_generate_image_task",
                {"prompt": prompt},
            )
            # If the server already returned an MCP-level error, surface it directly
            if "error" in create_resp:
                return create_resp

            try:
                # Extract "generate_image task_id: <id>" from the text content
                text = self._extract_text_from_mcp_result(create_resp) or ""
                prefix = "generate_image task_id:"
                if not text.startswith(prefix):
                    # Unexpected format; return as-is so caller can inspect
                    return create_resp
                task_id = text[len(prefix):].strip()
            except Exception:
                # If parsing fails, return original response so caller can debug
                return create_resp

            # 2) Poll generate-image task until completion or failure
            return self._poll_wan_task(
                "wan_query_generate_image_task",
                task_id,
            )

        # Default: Gemini
        return self.call_tool("gemini_generate_image", {"prompt": prompt})
    
    def edit_image(self, prompt: str, image_urls: list) -> Dict[str, Any]:
        """
        Edit images using the configured provider.
        
        For provider == "gemini":
            - Calls gemini_edit_image with a JSON array of image URLs / data URIs.
        
        For provider == "wan":
            - Wan only supports image URL input (no base64 or data URIs) and a single
              image URL per task. This method will:
                * Use the first URL in image_urls as image_url.
                * Call wan_create_edit_image_task to create an async task.
                * Extract task_id from the text result.
                * Call wan_query_edit_image_task once with the task_id and return that response.
        """
        if self.provider == "wan":
            if not image_urls:
                return {
                    "error": {
                        "code": -32602,
                        "message": "At least one image URL is required for Wan edit_image",
                    }
                }

            image_url = image_urls[0]

            # 1) Create edit-image task
            create_resp = self.call_tool(
                "wan_create_edit_image_task",
                {
                    "prompt": prompt,
                    "image_url": image_url,
                },
            )
            if "error" in create_resp:
                return create_resp

            try:
                # Extract "edit_image task_id: <id>" from the text content
                text = self._extract_text_from_mcp_result(create_resp) or ""
                prefix = "edit_image task_id:"
                if not text.startswith(prefix):
                    return create_resp
                task_id = text[len(prefix):].strip()
            except Exception:
                return create_resp

            # 2) Poll edit-image task until completion or failure
            return self._poll_wan_task(
                "wan_query_edit_image_task",
                task_id,
            )

        # Default: Gemini
        # Convert list to JSON string as required by the MCP tool
        image_urls_json = json.dumps(image_urls)
        return self.call_tool(
            "gemini_edit_image",
            {
                "prompt": prompt,
                "image_urls": image_urls_json,
            },
        )

