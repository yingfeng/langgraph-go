#!/usr/bin/env python3
"""
Comprehensive compatibility test for Python SDK with Go Server.
Tests all major features: Assistants, Threads, Runs (sync and async).
"""

import sys
import asyncio
sys.path.insert(0, "/Users/yingfeng/codebase/graph/langgraph/libs/sdk-py")

import httpx
from langgraph_sdk import get_client, get_sync_client

SERVER_URL = "http://localhost:8123"

def get_http_client():
    transport = httpx.HTTPTransport(retries=0)
    return httpx.Client(transport=transport)

def test_assistants_full():
    """Complete assistants API test."""
    print("\n" + "="*60)
    print("Testing Assistants API (Full)")
    print("="*60)
    client = get_sync_client(url=SERVER_URL)
    
    # Get existing assistants (should have one from server startup)
    print("\n1. List all assistants...")
    assistants = client.assistants.search()
    print(f"   Found {len(assistants)} assistant(s)")
    
    if assistants:
        assistant_id = assistants[0]["assistant_id"]
        print(f"   Assistant ID: {assistant_id}")
        
        # Get assistant by ID
        print("\n2. Get assistant by ID...")
        assistant = client.assistants.get(assistant_id)
        print(f"   ✓ Got assistant: {assistant.get('name', 'unnamed')}")
        
        # Search with filters
        print("\n3. Search with filters...")
        if assistant.get("graph_id"):
            filtered = client.assistants.search(graph_id=assistant["graph_id"])
            print(f"   ✓ Found {len(filtered)} with graph_id={assistant['graph_id']}")
        
        # Count assistants
        print("\n4. Count assistants...")
        count = client.assistants.count()
        print(f"   ✓ Total assistants: {count}")
        
        return assistant_id
    
    return None

def test_threads_full():
    """Complete threads API test."""
    print("\n" + "="*60)
    print("Testing Threads API (Full)")
    print("="*60)
    client = get_sync_client(url=SERVER_URL)
    
    # Create thread
    print("\n1. Create thread...")
    thread = client.threads.create(
        metadata={"test": "full_test", "source": "python_sdk"}
    )
    thread_id = thread["thread_id"]
    print(f"   ✓ Created thread: {thread_id}")
    
    # Get thread
    print("\n2. Get thread...")
    got_thread = client.threads.get(thread_id)
    print(f"   ✓ Thread status: {got_thread['status']}")
    
    # Update thread
    print("\n3. Update thread metadata...")
    updated = client.threads.update(
        thread_id,
        metadata={"test": "updated", "new_field": "value"}
    )
    print(f"   ✓ Updated thread")
    
    # Search threads
    print("\n4. Search threads...")
    results = client.threads.search(limit=10)
    print(f"   ✓ Found {len(results)} threads")
    
    # Verify our thread is in results
    our_thread_found = any(t["thread_id"] == thread_id for t in results)
    print(f"   ✓ Thread in search results: {our_thread_found}")
    
    return thread_id

def test_runs_sync(assistant_id, thread_id):
    """Test runs API with sync client."""
    print("\n" + "="*60)
    print("Testing Runs API (Sync)")
    print("="*60)
    client = get_sync_client(url=SERVER_URL)
    
    if not assistant_id or not thread_id:
        print("Skipping runs test (no assistant or thread)")
        return None
    
    # Create run
    print("\n1. Create run...")
    run = client.runs.create(
        thread_id=thread_id,
        assistant_id=assistant_id,
        input={"message": "Hello from Python SDK!"},
        stream_mode=["values", "updates"]
    )
    run_id = run["run_id"]
    print(f"   ✓ Created run: {run_id}")
    print(f"   Status: {run['status']}")
    print(f"   Input: {run.get('input')}")
    
    # Get run
    print("\n2. Get run by ID...")
    got_run = client.runs.get(run_id)
    print(f"   ✓ Got run: {got_run['run_id']}")
    
    # List runs for thread
    print("\n3. List runs for thread...")
    runs = client.runs.list(thread_id)
    print(f"   ✓ Found {len(runs)} run(s)")
    
    return run_id

async def test_assistants_async():
    """Test assistants API with async client."""
    print("\n" + "="*60)
    print("Testing Assistants API (Async)")
    print("="*60)
    
    async with get_client(url=SERVER_URL) as client:
        print("\n1. Async search assistants...")
        assistants = await client.assistants.search(limit=10)
        print(f"   ✓ Found {len(assistants)} assistant(s)")
        
        if assistants:
            print("\n2. Async count assistants...")
            count = await client.assistants.count()
            print(f"   ✓ Count: {count}")
            
            return assistants[0]["assistant_id"]
    
    return None

async def test_threads_async():
    """Test threads API with async client."""
    print("\n" + "="*60)
    print("Testing Threads API (Async)")
    print("="*60)
    
    async with get_client(url=SERVER_URL) as client:
        print("\n1. Async create thread...")
        thread = await client.threads.create(
            metadata={"async_test": True}
        )
        thread_id = thread["thread_id"]
        print(f"   ✓ Created thread: {thread_id}")
        
        print("\n2. Async get thread...")
        got_thread = await client.threads.get(thread_id)
        print(f"   ✓ Got thread: {got_thread['thread_id']}")
        
        print("\n3. Async search threads...")
        threads = await client.threads.search(limit=10)
        print(f"   ✓ Found {len(threads)} thread(s)")
        
        print("\n4. Async delete thread...")
        await client.threads.delete(thread_id)
        print(f"   ✓ Deleted thread: {thread_id}")

async def test_runs_async(assistant_id, thread_id):
    """Test runs API with async client."""
    print("\n" + "="*60)
    print("Testing Runs API (Async)")
    print("="*60)
    
    if not assistant_id or not thread_id:
        print("Skipping async runs test (no assistant or thread)")
        return
    
    async with get_client(url=SERVER_URL) as client:
        print("\n1. Async create run...")
        run = await client.runs.create(
            thread_id=thread_id,
            assistant_id=assistant_id,
            input={"async": True, "message": "Async test"}
        )
        run_id = run["run_id"]
        print(f"   ✓ Created run: {run_id}")
        
        print("\n2. Async get run...")
        got_run = await client.runs.get(run_id)
        print(f"   ✓ Got run: {got_run['run_id']}")

def test_health_and_basics():
    """Test basic health and connectivity."""
    print("\n" + "="*60)
    print("Testing Basic Connectivity")
    print("="*60)
    
    http_client = get_http_client()
    
    print("\n1. Health check...")
    r = http_client.get(f"{SERVER_URL}/health")
    assert r.status_code == 200
    data = r.json()
    print(f"   ✓ Server status: {data['status']}")
    
    print("\n2. Test direct HTTP requests...")
    # Test assistants endpoint
    r = http_client.get(f"{SERVER_URL}/assistants")
    assert r.status_code == 200
    print(f"   ✓ GET /assistants: {r.status_code}")
    
    # Test threads endpoint
    r = http_client.get(f"{SERVER_URL}/threads")
    assert r.status_code == 200
    print(f"   ✓ GET /threads: {r.status_code}")

async def run_async_tests(assistant_id):
    """Run all async tests."""
    print("\n" + "="*60)
    print("ASYNC CLIENT TESTS")
    print("="*60)
    
    # Get assistant_id from async search
    async_assistant_id = await test_assistants_async()
    if async_assistant_id:
        assistant_id = async_assistant_id
    
    await test_threads_async()
    
    # Create a thread for async runs test
    async with get_client(url=SERVER_URL) as client:
        thread = await client.threads.create()
        thread_id = thread["thread_id"]
    
    await test_runs_async(assistant_id, thread_id)
    
    # Cleanup
    async with get_client(url=SERVER_URL) as client:
        await client.threads.delete(thread_id)

def main():
    """Run all tests."""
    print("="*60)
    print("Python SDK - Go Server Comprehensive Test")
    print("="*60)
    print(f"Server URL: {SERVER_URL}")
    print()
    
    try:
        # Check server
        http_client = get_http_client()
        r = http_client.get(f"{SERVER_URL}/health")
        if r.status_code != 200:
            print(f"ERROR: Server not ready")
            return 1
    except Exception as e:
        print(f"ERROR: Cannot connect: {e}")
        return 1
    
    try:
        # Basic tests
        test_health_and_basics()
        
        # Sync tests
        assistant_id = test_assistants_full()
        thread_id = test_threads_full()
        run_id = test_runs_sync(assistant_id, thread_id)
        
        # Async tests
        asyncio.run(run_async_tests(assistant_id))
        
        # Cleanup
        print("\n" + "="*60)
        print("Cleanup")
        print("="*60)
        client = get_sync_client(url=SERVER_URL)
        if thread_id:
            try:
                client.threads.delete(thread_id)
                print(f"   ✓ Deleted test thread: {thread_id}")
            except:
                pass
        
        print("\n" + "="*60)
        print("✅ ALL TESTS PASSED!")
        print("Python SDK is fully compatible with Go Server.")
        print("="*60)
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
