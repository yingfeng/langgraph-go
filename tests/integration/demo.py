#!/usr/bin/env python3
"""
Demo: Complete workflow with assistant creation
Creates an assistant via HTTP API, then uses Python SDK for full workflow.
"""

import sys
import json
import httpx
sys.path.insert(0, "/Users/yingfeng/codebase/graph/langgraph/libs/sdk-py")

from langgraph_sdk import get_sync_client

SERVER_URL = "http://localhost:8123"

def get_http_client():
    transport = httpx.HTTPTransport(retries=0)
    return httpx.Client(transport=transport)

def main():
    print("=" * 70)
    print("Complete Workflow Demo with Assistant Creation")
    print("=" * 70)
    
    # Step 1: Create assistant via HTTP API
    print("\n[Step 1] Create Assistant via HTTP API")
    print("-" * 70)
    
    http_client = get_http_client()
    
    # Create assistant for the registered graph
    assistant_data = {
        "graph_id": "simple-graph",  # Must match registered graph in Go server
        "name": "Demo Assistant",
        "config": {
            "recursion_limit": 25
        },
        "metadata": {
            "created_by": "python_sdk",
            "demo": True
        }
    }
    
    r = http_client.post(
        f"{SERVER_URL}/assistants",
        json=assistant_data
    )
    
    if r.status_code == 201:
        assistant = r.json()
        assistant_id = assistant["assistant_id"]
        print(f"✓ Assistant created successfully!")
        print(f"  - ID: {assistant_id}")
        print(f"  - Name: {assistant['name']}")
        print(f"  - Graph: {assistant['graph_id']}")
    else:
        print(f"✗ Failed to create assistant: {r.status_code}")
        print(f"  Response: {r.text}")
        return 1
    
    # Step 2: Use Python SDK for full workflow
    print("\n[Step 2] Use Python SDK for Workflow")
    print("-" * 70)
    client = get_sync_client(url=SERVER_URL)
    
    # Verify assistant is visible
    print("Verifying assistant via SDK...")
    assistants = client.assistants.search()
    found = [a for a in assistants if a["assistant_id"] == assistant_id]
    if found:
        print(f"✓ Assistant visible via SDK: {found[0]['name']}")
    else:
        print(f"✗ Assistant not found via SDK")
        return 1
    
    # Step 3: Create thread
    print("\n[Step 3] Create Thread")
    print("-" * 70)
    thread = client.threads.create(
        metadata={"workflow": "assistant_demo"}
    )
    thread_id = thread["thread_id"]
    print(f"✓ Thread created: {thread_id}")
    
    # Step 4: Execute run
    print("\n[Step 4] Execute Run")
    print("-" * 70)
    run = client.runs.create(
        thread_id=thread_id,
        assistant_id=assistant_id,
        input={"message": "Hello from Python SDK!"},
        stream_mode=["values"]
    )
    run_id = run["run_id"]
    print(f"✓ Run created: {run_id}")
    print(f"  Status: {run['status']}")
    
    # Step 5: Monitor run completion
    print("\n[Step 5] Monitor Run Completion")
    print("-" * 70)
    import time
    for i in range(10):
        # Note: SyncRunsClient.get() requires both thread_id and run_id
        run = client.runs.get(thread_id, run_id)
        status = run['status']
        print(f"  [{i+1}s] Status: {status}")
        
        if status in ['success', 'error', 'interrupted']:
            if status == 'success':
                print(f"  ✓ Run completed successfully")
                if run.get('output'):
                    print(f"  Output: {json.dumps(run.get('output'), indent=2)}")
            break
        time.sleep(1)
    
    # Step 6: Get run details
    print("\n[Step 6] Get Run Details")
    print("-" * 70)
    final_run = client.runs.get(thread_id, run_id)
    print(f"✓ Run details:")
    print(f"  - ID: {final_run['run_id']}")
    print(f"  - Status: {final_run['status']}")
    print(f"  - Thread: {final_run['thread_id']}")
    print(f"  - Assistant: {final_run['assistant_id']}")
    if final_run.get('output'):
        print(f"  - Output: {json.dumps(final_run.get('output'), indent=4)}")
    
    # Step 7: List all runs for thread
    print("\n[Step 7] List Thread Runs")
    print("-" * 70)
    runs = client.runs.list(thread_id)
    print(f"✓ Total runs for thread: {len(runs)}")
    for i, r in enumerate(runs, 1):
        print(f"  [{i}] {r['run_id']}: {r['status']}")
    
    # Step 8: Search assistants
    print("\n[Step 8] Search Assistants")
    print("-" * 70)
    # Search by name
    results = client.assistants.search(name="Demo")
    print(f"✓ Found {len(results)} assistant(s) matching 'Demo'")
    
    # Search by graph_id
    results = client.assistants.search(graph_id="simple-graph")
    print(f"✓ Found {len(results)} assistant(s) with graph_id='simple-graph'")
    
    # Count assistants
    count = client.assistants.count()
    print(f"✓ Total assistants: {count}")
    
    # Step 9: Cleanup
    print("\n[Step 9] Cleanup (Optional)")
    print("-" * 70)
    
    # Uncomment to delete the test assistant
    # http_client.delete(f"{SERVER_URL}/assistants/{assistant_id}")
    # print(f"✓ Deleted assistant: {assistant_id}")
    
    # Delete the thread
    client.threads.delete(thread_id)
    print(f"✓ Deleted thread: {thread_id}")
    
    # Summary
    print("\n" + "=" * 70)
    print("✅ COMPLETE WORKFLOW DEMO SUCCESSFUL!")
    print("=" * 70)
    
    print("\nSummary:")
    print(f"  Created Assistant: {assistant_id}")
    print(f"  Created Thread: {thread_id}")
    print(f"  Created Run: {run_id}")
    print(f"  Run Status: {final_run['status']}")
    print(f"  API Calls Made:")
    print(f"    - Create assistant (HTTP)")
    print(f"    - Create thread (SDK)")
    print(f"    - Create run (SDK)")
    print(f"    - Get run (SDK)")
    print(f"    - List runs (SDK)")
    print(f"    - Search assistants (SDK)")
    print(f"    - Count assistants (SDK)")
    print(f"    - Delete thread (SDK)")
    
    print("\n" + "=" * 70)
    print("This demonstrates full compatibility between Python SDK and Go Server")
    print("=" * 70)
    
    return 0

if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception as e:
        print(f"\n✗ Error: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)
