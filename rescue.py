import json
import os

LOG_FILE = r"C:\Users\dka12\.gemini\antigravity\brain\0b342a93-f160-4b69-b49d-45d21158dc0f\.system_generated\logs\overview.txt"

TARGET_FILES = {
    "setup.html": "frontend/views/setup.html",
    "setup.go": "internal/api/setup.go",
    "sandbox.go": "internal/ledger/sandbox.go",
    "browser.go": "internal/api/browser.go",
    "sandbox_seed.sql": "internal/ledger/sandbox_seed.sql"
}

recovered = {}

print("Scanning log file...")
with open(LOG_FILE, "r", encoding="utf-8") as f:
    for line in f:
        if "{" not in line:
            continue
            
        json_str = line[line.index("{"):]
        try:
            data = json.loads(json_str)
        except json.JSONDecodeError:
            continue
            
        if "tool_calls" in data:
            for tc in data["tool_calls"]:
                if tc.get("name") == "write_to_file":
                    args = tc.get("args", {})
                    target = args.get("TargetFile", "")
                    content = args.get("CodeContent", "")
                    
                    for key, out_path in TARGET_FILES.items():
                        if key in target and content:
                            recovered[key] = content
                            
                elif tc.get("name") == "multi_replace_file_content":
                    args = tc.get("args", {})
                    target = args.get("TargetFile", "")
                    chunks = args.get("ReplacementChunks", [])
                    
                    for key, out_path in TARGET_FILES.items():
                        if key in target and key in recovered and chunks:
                            content = recovered[key]
                            for chunk in chunks:
                                t_content = chunk.get("TargetContent", "")
                                r_content = chunk.get("ReplacementContent", "")
                                if t_content in content:
                                    content = content.replace(t_content, r_content)
                            recovered[key] = content

for key, out_path in TARGET_FILES.items():
    if key in recovered:
        os.makedirs(os.path.dirname(out_path), exist_ok=True)
        with open(out_path, "w", encoding="utf-8") as f:
            f.write(recovered[key])
        print(f"Successfully recovered: {out_path}")
    else:
        print(f"FAILED to find: {key}")
