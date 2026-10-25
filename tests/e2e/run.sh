#!/usr/bin/env bash
# End-to-end check: builds the server and CLI, runs them with a stand-in for the Claude
# Code CLI, and clicks through the web UI. Needs Go and Node; installs jsdom locally.
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
root="$(cd "$here/../.." && pwd)"
work="$(mktemp -d)"
trap 'kill "$server" 2>/dev/null || true; rm -rf "$work"' EXIT
mkdir -p "$work/bin" "$work/repo" "$work/data"
go -C "$root" build -o "$work/bin/trireme" ./cmd/trireme
go -C "$root" build -o "$work/bin/trireme-cli" ./cmd/trireme-cli
cp "$here/fake-claude" "$work/bin/claude"
port=18799
cat > "$work/trireme.json" <<JSON
{"listen":"127.0.0.1:$port","data_dir":"$work/data","body_label":"e2e","claude_binary":"$work/bin/claude",
 "doctrine_dir":"$root/doctrine","doctrine_version":"v1",
 "projects":[{"id":"smoke","repo":"$work/repo","cap_usd":1,"seats":[{"id":"technical-lead"}]}]}
JSON
PATH="$work/bin:$PATH" "$work/bin/trireme" -config "$work/trireme.json" > "$work/server.log" 2>&1 &
server=$!
sleep 1.5
(cd "$here" && npm install --silent --no-audit --no-fund >/dev/null && node click.mjs "http://127.0.0.1:$port")

# Second server, requiring its API token as it does off the server's own machine.
kill "$server" 2>/dev/null || true
printf 'e2e-token-%s' "$RANDOM$RANDOM" > "$work/token"
sed -e "s|\"listen\":\"127.0.0.1:$port\"|\"listen\":\"127.0.0.1:$((port + 1))\",\"token_file\":\"$work/token\"|" \
    -e "s|$work/data|$work/data2|" "$work/trireme.json" > "$work/trireme-token.json"
mkdir -p "$work/data2"
PATH="$work/bin:$PATH" "$work/bin/trireme" -config "$work/trireme-token.json" >> "$work/server.log" 2>&1 &
server=$!
sleep 1.5
(cd "$here" && node token.mjs "http://127.0.0.1:$((port + 1))" "$(cat "$work/token")")
if grep -q '"level":"ERROR"' "$work/server.log"; then
  echo "server logged errors:"; grep '"level":"ERROR"' "$work/server.log"; exit 1
fi
