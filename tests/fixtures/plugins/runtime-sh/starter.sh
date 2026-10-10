#!/bin/sh
# Deliberately not executable: the fixture proves Work runs it through its
# declared runtime instead of relying on a shebang or an exec bit.
cat > /dev/null
printf '{"repository":{"name":"runtime-sh-repo"}}\n'
