import json
import sys
request = json.load(sys.stdin)
# Replace with real repository resolution for the matched argument.
# Optional modes can overlay links/meta for new, contribution and fork;
# see references/component-roles.md. Validate every entry, even if unselected.
print(json.dumps({"repository": {"name": request["arg"].removeprefix("example:")}}))
