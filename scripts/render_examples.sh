#!/usr/bin/env bash
# Re-render every example directory under examples/. For each directory
# containing a single <name>.d2 file, write:
#   <name>.dagre.{svg,png}     — dagre baseline
#   <name>.graphviz.{svg,png}  — graphviz on the same source
# If a <name>.tuned.d2 sibling exists, additionally write:
#   <name>.tuned.{svg,png}     — graphviz on the tuned source
#
# Used to regenerate the README example images after editing any
# example source. Requires d2 and the d2plugin-graphviz binary on
# $PATH (run scripts/eval.sh once to install the plugin if needed).
set -euo pipefail

HERE="$(cd "$(dirname "$0")/.." && pwd)"
cd "$HERE"

# Install/rebuild the plugin in case the source has changed.
GOBIN="${GOBIN:-$HOME/.local/bin}"
mkdir -p "$GOBIN"
echo ">> installing d2plugin-graphviz to $GOBIN"
GOBIN="$GOBIN" go install ./cmd/d2plugin-graphviz

render() {
	local layout="$1" src="$2" out="$3"
	echo "   $layout  $src -> $out"
	d2 --layout "$layout" "$src" "$out" >/dev/null 2>&1
}

for dir in examples/*/; do
	# Skip the legacy top-level examples (Hera input.d2 / input.tuned.d2).
	[[ -d "$dir" ]] || continue
	# Find base .d2 files, ignoring .tuned.d2 siblings.
	for base in "$dir"*.d2; do
		[[ -f "$base" ]] || continue
		case "$base" in
			*.tuned.d2) continue ;;
		esac
		stem="${base%.d2}"
		name="$(basename "$stem")"
		echo ">> $dir$name"
		render dagre    "$base" "$stem.dagre.svg"
		render dagre    "$base" "$stem.dagre.png"
		render graphviz "$base" "$stem.graphviz.svg"
		render graphviz "$base" "$stem.graphviz.png"
		tuned="$stem.tuned.d2"
		if [[ -f "$tuned" ]]; then
			render graphviz "$tuned" "$stem.tuned.svg"
			render graphviz "$tuned" "$stem.tuned.png"
		fi
	done
done

echo ">> done"
