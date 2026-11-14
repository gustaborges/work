import json
import sys
request = json.load(sys.stdin)
# Inspect configured roots and return actual Git repository paths here.
print(json.dumps({"matches": []}))
