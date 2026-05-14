#!/usr/bin/env bash
# Render every .dot in this directory to paired light/dark SVGs.
# Requires Graphviz `dot` in $PATH; the rule is a no-op if dot is missing.
set -euo pipefail
cd "$(dirname "$0")"
if ! command -v dot >/dev/null 2>&1; then
    echo "graphviz dot not found; skipping render"
    exit 0
fi
for src in *.dot; do
    [ -e "$src" ] || continue
    name="${src%.dot}"
    dot -Tsvg "$src" -o "${name}-light.svg"
    dot -Tsvg \
        -Gbgcolor=transparent \
        -Nfontcolor="#F1F5F9" \
        -Ecolor="#94A3B8" \
        -Efontcolor="#94A3B8" \
        "$src" -o "${name}-dark.svg"
done
