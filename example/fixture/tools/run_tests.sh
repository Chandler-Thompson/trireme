#!/usr/bin/env bash
# The fixture's whole test surface, in one command.
#
# A Trireme fixture needs exactly this: one command, exit 0 for green and
# non-zero for red, no arguments, runnable from the repository root. The
# merge gate runs it, the Verifier runs it, and the Worker's evidence line
# quotes it. If a project cannot offer one command, give it a wrapper script
# that is one command.
set -euo pipefail
cd "$(dirname "$0")/.."
exec python3 -m unittest discover -s tests -t . -q
