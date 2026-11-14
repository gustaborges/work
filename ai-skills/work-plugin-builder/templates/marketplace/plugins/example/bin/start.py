import json
import sys
request = json.load(sys.stdin)
# Replace with real repository resolution for the matched argument.
print(json.dumps({"repository": {"name": request["arg"].removeprefix("example:")}}))
