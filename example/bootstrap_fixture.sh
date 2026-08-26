#!/usr/bin/env bash
# Copy example/fixture into a scratch directory and make it a git repository.
#
# The fixture ships as plain files rather than a nested git repo, because a
# repository inside a repository is a submodule argument nobody wants to have.
# This script turns it into the real thing the sortie needs: a git repo with a
# green main branch, which the harness can cut task worktrees from.
set -euo pipefail

SRC="$(cd "$(dirname "$0")/fixture" && pwd)"
DEST="${TRIREME_EXAMPLE_FIXTURE:-/tmp/trireme-example-fixture}"

if [ -e "$DEST" ]; then
  echo "removing existing $DEST"
  rm -rf "$DEST"
fi

mkdir -p "$DEST"
cp -r "$SRC"/. "$DEST"/
cd "$DEST"

git init -q -b main
git add -A
git -c user.name=trireme-example -c user.email=example@trireme.local \
    commit -q -m "Interval library fixture"

echo "fixture ready at $DEST"
bash tools/run_tests.sh
echo "suite green — ready to fly:"
echo "  python3 example/sortie_example.py"
