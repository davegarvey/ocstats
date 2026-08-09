#!/bin/bash
# <xbar.title>OpenCode Go Quota</xbar.title>
# <xbar.version>v1.0</xbar.version>
# <xbar.author>ocstats</xbar.author>
# <xbar.desc>Shows the opencode.ai Go subscription quota (rolling/weekly/monthly) with period-anchored projections.</xbar.desc>
# <xbar.abouturl>https://opencode.ai/docs/go/</xbar.abouturl>
# <swiftbar.run-in-bash>true</swiftbar.run-in-bash>
# ocstats SwiftBar plugin - shows opencode Go quota in the macOS menu bar.
# Refreshes every 15 minutes (name suffix ".15m"). Requires `ocstats` on PATH.
# No secrets here: everything flows through `ocstats quota --json`.
BIN="$(command -v ocstats)"
if [ -z "$BIN" ] && [ -x "$HOME/.local/bin/ocstats" ]; then
  BIN="$HOME/.local/bin/ocstats"
fi
if [ -z "$BIN" ]; then
  echo "ocstats not found | color=red"
  echo "---"
  echo "Install ocstats and put it on PATH | color=red"
  exit 0
fi

OUT="$("$BIN" quota --json 2>/dev/null)"
EXIT=$?
if [ -z "$OUT" ]; then
  echo "Go quota unavailable | color=red"
  echo "---"
  echo "Fetch failed (exit $EXIT) | color=red"
  echo "Run ocstats login | bash=ocstats param1=login terminal=false"
  exit 0
fi

python3 - "$OUT" "$EXIT" <<'PYEOF'
import json, sys, datetime

try:
    data = json.loads(sys.argv[1])
except Exception:
    data = {}

if "error" in data or "rolling" not in data:
    print("Go quota unavailable | color=red")
    print("---")
    print(f"Fetch failed (exit {sys.argv[2]}) | color=red")
    print("Run ocstats login | bash=ocstats param1=login terminal=false")
    sys.exit(0)

def human(secs):
    secs = max(0, int(secs))
    if secs < 60: return f"{secs}s"
    m = secs // 60
    if m < 60: return f"{m}m"
    h = m // 60
    if h < 24: return f"{h}h {m % 60}m"
    d = h // 24
    return f"{d}d {h % 24}h"

if data.get("subscription") is None:
    print("No Go sub | color=gray")
    print("---")
    print(f"No Go subscription for {data.get('workspaceId','?')} | color=gray")
    sys.exit(0)

roll = data["rolling"]
w = data["weekly"]
m = data["monthly"]

def projected(bucket):
    period = bucket.get("periodLengthSec")
    remaining = bucket.get("resetInSec")
    if period is None or not remaining:
        return None
    elapsed = period - remaining
    if elapsed <= 0:
        return None
    return round(bucket["usagePercent"] * period / elapsed)

over = any(p is not None and p >= 100 for p in (projected(roll), projected(w), projected(m)))
icon = "exclamationmark.triangle" if over else "gauge"
print(f" | sfimage={icon}")
print("---")

def projection(bucket):
    p = projected(bucket)
    return f"  → {p}% by reset" if p is not None else ""

print(f"Rolling  {roll['usagePercent']}%  resets in {human(roll['resetInSec'])}{projection(roll)}")
print(f"Weekly   {w['usagePercent']}%  resets in {human(w['resetInSec'])}{projection(w)}")
print(f"Monthly  {m['usagePercent']}%  resets in {human(m['resetInSec'])}{projection(m)}")
print("---")
print(f"Fetched {data['fetchedAt']} | size=10 color=gray")
ws = data.get("workspaceId", "")
if ws:
    print(f"Open website | href=https://opencode.ai/workspace/{ws}/go")
print("Refresh | refresh=true")
PYEOF
