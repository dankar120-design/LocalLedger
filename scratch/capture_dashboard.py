import sys
import os
import json
import time
import base64
import urllib.request
from websocket import create_connection

def main():
    # 1. Fetch the debugging targets from port 9222
    try:
        req = urllib.request.urlopen("http://127.0.0.1:9222/json")
        targets = json.loads(req.read().decode())
    except Exception as e:
        print(f"Error: Could not connect to Chrome debugger on port 9222. Is Chrome running? Details: {e}")
        sys.exit(1)
        
    page_target = None
    for t in targets:
        if t.get('type') == 'page':
            page_target = t
            break
            
    if not page_target:
        print("Error: No active page target found in Chrome debugger.")
        sys.exit(1)
        
    ws_url = page_target['webSocketDebuggerUrl']
    print(f"Connecting to Chrome debugger at: {ws_url}")
    
    ws = create_connection(ws_url, suppress_origin=True)
    
    msg_id = 1
    
    def send_cmd(method, params=None):
        nonlocal msg_id
        payload = {
            "id": msg_id,
            "method": method,
            "params": params or {}
        }
        msg_id += 1
        ws.send(json.dumps(payload))
        
        # Wait for the matching response
        while True:
            resp = json.loads(ws.recv())
            if resp.get('id') == payload['id']:
                if 'error' in resp:
                    raise Exception(f"CDP Command Error: {resp['error']}")
                return resp.get('result')

    # Enable page notifications and navigation
    send_cmd("Page.enable")
    
    # 2. Navigate to LocalLedger
    print("Navigating to http://127.0.0.1:8080/ ...")
    send_cmd("Page.navigate", {"url": "http://127.0.0.1:8080/"})
    
    # Wait for page load
    print("Waiting for page load...")
    time.sleep(3.5)
    
    artifacts_dir = r"C:\Users\dka12\.gemini\antigravity\brain\7b59ca5a-3630-4c6e-af81-0f7943482e79"
    os.makedirs(artifacts_dir, exist_ok=True)
    
    # Screenshot helper
    def take_screenshot(filename):
        print(f"Capturing screenshot: {filename}...")
        res = send_cmd("Page.captureScreenshot", {"format": "png"})
        img_data = base64.b64decode(res['data'])
        filepath = os.path.join(artifacts_dir, filename)
        with open(filepath, "wb") as f:
            f.write(img_data)
        print(f"Saved: {filepath} ({len(img_data)} bytes)")
        
    def eval_js(expression):
        res = send_cmd("Runtime.evaluate", {
            "expression": expression,
            "returnByValue": True
        })
        if 'exceptionDetails' in res:
            print(f"JS Exception for '{expression}': {res['exceptionDetails']}")
        else:
            print(f"JS Eval '{expression}': {res.get('result', {})}")
        return res.get('result', {}).get('value')

    # Capture 1: Dashboard View
    take_screenshot("dashboard_view.png")
    
    # Switch to Verktyg (Tools & Export)
    print("Switching view to 'verktyg'...")
    eval_js("document.body._x_dataStack[0].showView('verktyg')")
    time.sleep(1.5)
    take_screenshot("tools_view.png")
    
    # Switch to Settings (Inställningar)
    print("Switching view to 'installningar'...")
    eval_js("document.body._x_dataStack[0].showView('installningar')")
    time.sleep(2.0)
    take_screenshot("settings_view.png")
    
    # Show Shutdown overlay (without actually triggering backend kill yet)
    print("Activating shutdown overlay visual...")
    eval_js("document.body._x_dataStack[0].showShutdown = true")
    time.sleep(1.5)
    take_screenshot("shutdown_view.png")
    
    # All done!
    print("E2E Visual Screenshots Captured Successfully!")
    ws.close()

if __name__ == "__main__":
    main()
