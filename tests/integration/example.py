#!/usr/bin/env python3
"""
Example: Using Python SDK with Go LangGraph Server

This demonstrates a complete workflow:
1. Connect to Go server
2. Create an assistant (for a registered graph)
3. Create a thread
4. Execute a run
5. Stream results
"""

import asyncio
import sys
sys.path.insert(0, "/Users/yingfeng/codebase/graph/langgraph/libs/sdk-py")

from langgraph_sdk import get_client, get_sync_client

SERVER_URL = "http://localhost:8123"

def main():
    print("=" * 70)
    print("Python SDK + Go LangGraph Server Example")
    print("=" * 70)
    
    # Connect to Go server
    print("\n[1] Connecting to Go Server...")
    client = get_sync_client(url=SERVER_URL)
    print(f"    Connected to: {SERVER_URL}")
    
    # Check server health
    print("\n[2] Checking Server Health...")
    import httpx
    transport = httpx.HTTPTransport(retries=0)
    http_client = httpx.Client(transport=transport)
    
    r = http_client.get(f"{SERVER_URL}/health")
    if r.status_code == 200:
        print(f"    ✓ Server is healthy: {r.json()}")
    else:
        print(f"    ✗ Server returned status {r.status_code}")
        return 1
    
    # List available assistants
    print("\n[3] Listing Available Assistants...")
    assistants = client.assistants.search()
    if assistants:
        print(f"    Found {len(assistants)} assistant(s):")
        for i, asst in enumerate(assistants, 1):
            print(f"      {i}. {asst.get('name', 'unnamed')} (ID: {asst['assistant_id']})")
            print(f"         Graph: {asst.get('graph_id', 'unknown')}")
    else:
        print("    ℹ No assistants found")
        print("    ℹ In Go server, graphs are pre-registered")
        print("    ℹ Assistants can be created via API if needed")
        
        # Create an assistant for demo
        print("\n    Creating demo assistant...")
        # Note: This requires a registered graph in the Go server
        # For this example, we'll skip assistant creation
        # In production: client.assistants.create(graph_id="your-graph-id", name="My Assistant")
    
    if not assistants:
        print("\n[4] Skipping run test (no assistant available)")
        print("=" * 70)
        print("Setup Complete!")
        print("=" * 70)
        print("\nNext Steps:")
        print("  1. Register a graph in your Go server")
        print("  2. Create an assistant for that graph via API")
        print("  3. Create threads and execute runs")
        print("\nExample assistant creation:")
        print("""
    # Via HTTP API
    curl -X POST http://localhost:8123/assistants \\
      -H "Content-Type: application/json" \\
      -d '{
        "graph_id": "your-graph-id",
        "name": "My Assistant",
        "config": {"recursion_limit": 25}
      }'
    
    # Via Python SDK
    client = get_sync_client(url="http://localhost:8123")
    assistant = client.assistants.create(
        graph_id="your-graph-id",
        name="My Assistant"
    )
        """)
        return 0
    
    # Use the first assistant
    assistant = assistants[0]
    assistant_id = assistant["assistant_id"]
    print(f"\n[4] Using Assistant: {assistant.get('name', 'unnamed')}")
    print(f"    ID: {assistant_id}")
    
    # Create a thread
    print("\n[5] Creating Conversation Thread...")
    thread = client.threads.create(
        metadata={"demo": True, "source": "python_sdk"}
    )
    thread_id = thread["thread_id"]
    print(f"    ✓ Created thread: {thread_id}")
    print(f"    Status: {thread['status']}")
    
    # Execute a run
    print("\n[6] Executing Graph Run...")
    print("    Input: {'message': 'Hello from Python SDK!'}")
    
    run = client.runs.create(
        thread_id=thread_id,
        assistant_id=assistant_id,
        input={"message": "Hello from Python SDK!"},
        stream_mode=["values", "updates"]
    )
    run_id = run["run_id"]
    print(f"    ✓ Created run: {run_id}")
    print(f"    Status: {run['status']}")
    
    # Wait for run completion
    print("\n[7] Waiting for Run Completion...")
    import time
    max_wait = 5
    for i in range(max_wait):
        run = client.runs.get(run_id)
        status = run['status']
        print(f"    [{i+1}s] Status: {status}")
        
        if status in ['success', 'error', 'interrupted']:
            print(f"    ✓ Run completed: {status}")
            if status == 'success' and run.get('output'):
                print(f"    Output: {run.get('output')}")
            break
        time.sleep(1)
    else:
        print(f"    ℹ Run still in progress after {max_wait}s")
    
    # List runs for the thread
    print("\n[8] Listing Thread Runs...")
    runs = client.runs.list(thread_id)
    print(f"    Found {len(runs)} run(s)")
    for i, r in enumerate(runs, 1):
        print(f"      {i}. {r['run_id']} - {r['status']}")
    
    # Update thread metadata
    print("\n[9] Updating Thread Metadata...")
    updated = client.threads.update(
        thread_id,
        metadata={
            "demo": True,
            "runs_completed": len(runs),
            "last_status": run['status']
        }
    )
    print(f"    ✓ Thread updated: {updated['thread_id']}")
    
    print("\n" + "=" * 70)
    print("✅ Workflow Completed Successfully!")
    print("=" * 70)
    
    print("\nSummary:")
    print(f"  Assistant: {assistant.get('name', 'unnamed')}")
    print(f"  Thread: {thread_id}")
    print(f"  Runs: {len(runs)}")
    print(f"  Final Status: {run['status']}")
    
    print("\nCleanup:")
    print(f"  # To delete the thread:")
    print(f"  client.threads.delete('{thread_id}')")
    
    print("\n" + "=" * 70)
    print("For more information:")
    print("  - Go Server: https://github.com/infiniflow/ragflow/agent")
    print("  - Python SDK: https://github.com/langchain-ai/langgraph")
    print("=" * 70)
    
    return 0

async def async_example():
    """Async version of the example."""
    print("\n" + "=" * 70)
    print("Async Example")
    print("=" * 70)
    
    async with get_client(url=SERVER_URL) as client:
        print("\n[Async] Creating thread...")
        thread = await client.threads.create()
        thread_id = thread["thread_id"]
        print(f"    ✓ Created: {thread_id}")
        
        print("\n[Async] Getting thread...")
        got = await client.threads.get(thread_id)
        print(f"    ✓ Retrieved: {got['thread_id']}")
        
        # Cleanup
        print("\n[Async] Deleting thread...")
        await client.threads.delete(thread_id)
        print(f"    ✓ Deleted: {thread_id}")

if __name__ == "__main__":
    try:
        # Run sync example
        result = main()
        
        # Run async example
        print("\n" + "=" * 70)
        asyncio.run(async_example())
        
        sys.exit(result)
        
    except Exception as e:
        print(f"\nError: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)
