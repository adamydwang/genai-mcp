#!/usr/bin/env python3
"""
Run all MCP server tests in sequence
"""
import sys
import subprocess
import time


def run_test(script_name, *args):
    """Run a test script and return the exit code"""
    cmd = ["python3", script_name] + list(args)
    print(f"\n{'='*60}")
    print(f"Running: {' '.join(cmd)}")
    print('='*60)
    
    result = subprocess.run(cmd)
    return result.returncode


def main():
    """Run all tests"""
    base_url = sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:8080/mcp"
    
    print("="*60)
    print("Running All MCP Server Tests")
    print("="*60)
    print(f"Server URL: {base_url}")
    
    tests_passed = 0
    tests_failed = 0
    
    # Test 1: List tools
    print("\n[Test 1/3] Listing all tools...")
    if run_test("test_list_tools.py", base_url) == 0:
        tests_passed += 1
        print("✅ Test 1 passed")
    else:
        tests_failed += 1
        print("❌ Test 1 failed")
    
    time.sleep(1)
    
    # Test 2: Generate image
    print("\n[Test 2/3] Testing image generation...")
    test_prompt = "A beautiful sunset over mountains"
    if run_test("test_generate_image.py", test_prompt, base_url) == 0:
        tests_passed += 1
        print("✅ Test 2 passed")
    else:
        tests_failed += 1
        print("❌ Test 2 failed")
    
    time.sleep(1)
    
    # Test 3: Edit image (using a sample image URL)
    print("\n[Test 3/3] Testing image editing...")
    print("Note: This test requires a valid image URL.")
    print("You can skip this test if you don't have an image URL to test with.")
    
    # For demonstration, we'll use a placeholder
    # In real usage, you would use an actual image URL from test 2
    sample_image_url = "https://example.com/sample.jpg"
    edit_prompt = "Make it more colorful"
    
    response = input(f"\nUse sample URL '{sample_image_url}' for testing? (y/n): ")
    if response.lower() == 'y':
        if run_test("test_edit_image.py", edit_prompt, sample_image_url, base_url) == 0:
            tests_passed += 1
            print("✅ Test 3 passed")
        else:
            tests_failed += 1
            print("❌ Test 3 failed")
    else:
        print("⏭️  Test 3 skipped")
    
    # Summary
    print("\n" + "="*60)
    print("Test Summary")
    print("="*60)
    print(f"✅ Passed: {tests_passed}")
    print(f"❌ Failed: {tests_failed}")
    print(f"⏭️  Skipped: {3 - tests_passed - tests_failed}")
    print("="*60)
    
    return 0 if tests_failed == 0 else 1


if __name__ == "__main__":
    sys.exit(main())

