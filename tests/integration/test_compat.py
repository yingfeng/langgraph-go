#!/usr/bin/env python3
"""
Quick API compatibility test between Python SDK and Go Server.
Usage:
    1. Start Go server: go run ./server/example/main.go
    2. Run this test: python3 test_compat.py
"""

import sys
import json
sys.path.insert(0, "/Users/yingfeng/codebase/graph/langgraph/libs/sdk-py")

import httpx

SERVER_URL = "http://localhost:8123"

# Create httpx client with custom transport (no retries to avoid 502 errors)
def get_http_client():
    transport = httpx.HTTPTransport(retries=0)
    return httpx.Client(transport=transport)

def test_health():
    """Test health endpoint."""
    print("Testing /health...")
    client = get_http_client()
    r = client.get(f"{SERVER_URL}/health")
    assert r.status_code == 200, f"Expected 200, got {r.status_code}"
    data = r.json()
    assert data.get("status") == "ok", f"Expected status 'ok', got {data}"
    print(f"  ✓ Health check passed: {data}")

def test_assistants_api():
    """Test assistants API."""
    print("\nTesting Assistants API...")
    client = get_http_client()
    
    # Test GET /assistants (list)
    print("  Testing GET /assistants...")
    r = client.get(f"{SERVER_URL}/assistants")
    assert r.status_code == 200, f"Expected 200, got {r.status_code}"
    assistants = r.json()
    assert isinstance(assistants, list), f"Expected list, got {type(assistants)}"
    print(f"    ✓ List assistants: {len(assistants)} assistants")
    
    # Test POST /assistants/search
    print("  Testing POST /assistants/search...")
    r = client.post(f"{SERVER_URL}/assistants/search", json={"limit": 10})
    assert r.status_code == 200, f"Expected 200, got {r.status_code}: {r.text}"
    search_result = r.json()
    # Should return a list directly
    assert isinstance(search_result, list), f"Expected list, got {type(search_result)}: {search_result}"
    print(f"    ✓ Search assistants: {len(search_result)} results")
    
    # Test POST /assistants/count
    print("  Testing POST /assistants/count...")
    r = client.post(f"{SERVER_URL}/assistants/count", json={})
    assert r.status_code == 200, f"Expected 200, got {r.status_code}: {r.text}"
    count = r.json()
    assert isinstance(count, int), f"Expected int, got {type(count)}: {count}"
    print(f"    ✓ Count assistants: {count}")

def test_threads_api():
    """Test threads API."""
    print("\nTesting Threads API...")
    client = get_http_client()
    
    # Test POST /threads (create)
    print("  Testing POST /threads...")
    r = client.post(f"{SERVER_URL}/threads", json={})
    assert r.status_code == 201, f"Expected 201, got {r.status_code}: {r.text}"
    thread = r.json()
    assert "thread_id" in thread, f"Missing thread_id in response: {thread}"
    thread_id = thread["thread_id"]
    print(f"    ✓ Created thread: {thread_id}")
    
    # Test GET /threads/{thread_id}
    print("  Testing GET /threads/{thread_id}...")
    r = client.get(f"{SERVER_URL}/threads/{thread_id}")
    assert r.status_code == 200, f"Expected 200, got {r.status_code}: {r.text}"
    got_thread = r.json()
    assert got_thread["thread_id"] == thread_id
    print(f"    ✓ Got thread: {got_thread['thread_id']}")
    
    # Test PATCH /threads/{thread_id}
    print("  Testing PATCH /threads/{thread_id}...")
    r = client.patch(f"{SERVER_URL}/threads/{thread_id}", json={
        "metadata": {"test_key": "test_value"}
    })
    assert r.status_code == 200, f"Expected 200, got {r.status_code}: {r.text}"
    updated = r.json()
    print(f"    ✓ Updated thread: {updated['thread_id']}")
    
    # Test POST /threads/search
    print("  Testing POST /threads/search...")
    r = client.post(f"{SERVER_URL}/threads/search", json={"limit": 10})
    assert r.status_code == 200, f"Expected 200, got {r.status_code}: {r.text}"
    threads = r.json()
    assert isinstance(threads, list), f"Expected list, got {type(threads)}"
    print(f"    ✓ Search threads: {len(threads)} results")
    
    # Test DELETE /threads/{thread_id}
    print("  Testing DELETE /threads/{thread_id}...")
    r = client.delete(f"{SERVER_URL}/threads/{thread_id}")
    assert r.status_code == 204, f"Expected 204, got {r.status_code}: {r.text}"
    print(f"    ✓ Deleted thread: {thread_id}")

def test_sdk_client():
    """Test Python SDK client."""
    print("\nTesting Python SDK client...")
    
    from langgraph_sdk import get_sync_client
    
    # Create client
    client = get_sync_client(url=SERVER_URL)
    print("  ✓ Created SDK client")
    
    # Test assistants.search
    print("  Testing client.assistants.search()...")
    assistants = client.assistants.search(limit=10)
    assert isinstance(assistants, list), f"Expected list, got {type(assistants)}"
    print(f"    ✓ Found {len(assistants)} assistants")
    
    # Test assistants.count
    print("  Testing client.assistants.count()...")
    count = client.assistants.count()
    assert isinstance(count, int), f"Expected int, got {type(count)}"
    print(f"    ✓ Count: {count}")
    
    # Test threads.create
    print("  Testing client.threads.create()...")
    thread = client.threads.create()
    assert "thread_id" in thread
    thread_id = thread["thread_id"]
    print(f"    ✓ Created thread: {thread_id}")
    
    # Test threads.get
    print("  Testing client.threads.get()...")
    got_thread = client.threads.get(thread_id)
    assert got_thread["thread_id"] == thread_id
    print(f"    ✓ Got thread: {got_thread['thread_id']}")
    
    # Test threads.search
    print("  Testing client.threads.search()...")
    threads = client.threads.search(limit=10)
    assert isinstance(threads, list)
    print(f"    ✓ Found {len(threads)} threads")
    
    # Cleanup
    print("  Cleaning up...")
    client.threads.delete(thread_id)
    print("    ✓ Deleted test thread")

def main():
    """Run all tests."""
    print("=" * 60)
    print("Python SDK - Go Server API Compatibility Test")
    print("=" * 60)
    print(f"Server URL: {SERVER_URL}")
    print()
    
    try:
        # First check if server is running
        client = get_http_client()
        r = client.get(f"{SERVER_URL}/health")
        if r.status_code != 200:
            print(f"ERROR: Server not ready (status {r.status_code})")
            return 1
    except Exception as e:
        print(f"ERROR: Cannot connect to server at {SERVER_URL}")
        print(f"  {e}")
        print("\nMake sure the Go server is running:")
        print("  go run ./server/example/main.go")
        return 1
    
    try:
        # Run tests
        test_health()
        test_assistants_api()
        test_threads_api()
        test_sdk_client()
        
        print("\n" + "=" * 60)
        print("All tests passed! Python SDK is compatible with Go Server.")
        print("=" * 60)
        return 0
        
    except AssertionError as e:
        print(f"\n✗ Test failed: {e}")
        return 1
    except Exception as e:
        print(f"\n✗ Error: {e}")
        import traceback
        traceback.print_exc()
        return 1

if __name__ == "__main__":
    sys.exit(main())
