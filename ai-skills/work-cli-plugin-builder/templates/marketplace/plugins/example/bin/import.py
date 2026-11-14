import json
import pathlib
import sys
request = json.load(sys.stdin)
# Write only inside the core-provided output_dir. The core incorporates files.
(pathlib.Path(request["output_dir"]) / "example.txt").write_text(
    str(request["inputs"].get("slug", "example")), encoding="utf-8")
print("{}")
