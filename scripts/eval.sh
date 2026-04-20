#!/usr/bin/env bash
# Render examples/input.d2 (or any .d2 file passed as $1) with dagre and
# graphviz layouts side-by-side, then produce out/compare.html for visual
# diff in a browser. Rebuilds and re-installs the plugin each run.
set -euo pipefail

HERE="$(cd "$(dirname "$0")/.." && pwd)"
cd "$HERE"

INPUT="${1:-examples/input.d2}"
OUTDIR="${2:-out}"
mkdir -p "$OUTDIR"

# Use the GOBIN that already lives on $PATH so D2 can discover the plugin.
GOBIN="${GOBIN:-$HOME/.local/bin}"
mkdir -p "$GOBIN"
echo ">> installing d2plugin-graphviz to $GOBIN"
GOBIN="$GOBIN" go install ./cmd/d2plugin-graphviz

stem() { basename "${1%.*}"; }
base=$(stem "$INPUT")

render() {
	local layout="$1"
	local ext="$2"
	local out="$OUTDIR/${base}.${layout}.${ext}"
	echo ">> rendering $layout $ext"
	d2 --layout "$layout" "$INPUT" "$out" 2>&1 | sed 's/^/    /'
}

render dagre svg
render graphviz svg
render dagre png
render graphviz png

# If a tuned variant exists alongside the input, render it too so the
# comparison page shows untuned vs tuned side-by-side.
TUNED="${INPUT%.d2}.tuned.d2"
HAS_TUNED=0
if [[ -f "$TUNED" ]]; then
	HAS_TUNED=1
	echo ">> rendering tuned"
	d2 --layout graphviz "$TUNED" "$OUTDIR/${base}.tuned.graphviz.svg" 2>&1 | sed 's/^/    /'
	d2 --layout graphviz "$TUNED" "$OUTDIR/${base}.tuned.graphviz.png" 2>&1 | sed 's/^/    /'
fi

# Side-by-side HTML
{
	echo '<!doctype html>'
	echo "<title>dagre vs graphviz — $(stem "$INPUT")</title>"
	cat <<'CSS'
<style>
	body{margin:0;font-family:system-ui,sans-serif;background:#111;color:#eee}
	header{padding:12px 16px;background:#222;border-bottom:1px solid #333}
	.grid{display:grid;gap:8px;padding:8px;align-items:start}
	.cols-2{grid-template-columns:1fr 1fr}
	.cols-3{grid-template-columns:1fr 1fr 1fr}
	.panel{background:#fff;border-radius:4px;padding:8px}
	.panel h3{margin:0 0 8px;color:#333;font:600 13px system-ui}
	.panel img{width:100%;height:auto;display:block}
</style>
CSS
	echo "<header><strong>$(stem "$INPUT")</strong> — layout comparison"
	echo "(rebuild &amp; rerun: <code>scripts/eval.sh $INPUT</code>)</header>"
	if [[ $HAS_TUNED -eq 1 ]]; then
		echo '<div class="grid cols-3">'
	else
		echo '<div class="grid cols-2">'
	fi
	echo "<div class=\"panel\"><h3>dagre (bundled)</h3><img src=\"${base}.dagre.svg\"></div>"
	echo "<div class=\"panel\"><h3>graphviz (untuned)</h3><img src=\"${base}.graphviz.svg\"></div>"
	if [[ $HAS_TUNED -eq 1 ]]; then
		echo "<div class=\"panel\"><h3>graphviz (tuned)</h3><img src=\"${base}.tuned.graphviz.svg\"></div>"
	fi
	echo '</div>'
} > "$OUTDIR/compare.html"

echo ">> wrote $OUTDIR/compare.html"
