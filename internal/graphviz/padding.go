// Plugin-only cluster attributes for CSS-style padding and rank-anchor
// alignment between clusters. These keys are consumed by the plugin
// before the remaining ClusterAttrs entries are passed to dot.
//
// Supported keys:
//
//   padding_top:        invisible vertical space above the first visible
//                       node inside the cluster. Value is a numeric
//                       string in inches (e.g. "0.6"). Implemented as a
//                       zero-width invisible node at the top of the
//                       cluster with explicit height plus a high-weight
//                       constraint edge to the first leaf.
//
//   padding_bottom:     mirror of padding_top, on the bottom side.
//
//   align_top_with:     AbsID of another cluster. Forces the first leaf
//                       of this cluster to share a rank with the first
//                       leaf of the target cluster. Implemented as a
//                       top-level `{rank=same; ...}` group.
//
//   align_bottom_with:  mirror of align_top_with on the last leaf.
//
// Why these are plugin-only rather than passthrough: dot has no native
// per-side cluster margin (cluster `margin` is a scalar) and no
// position attribute for ranking a cluster relative to another. The
// effects we want are mechanical compositions of invisible nodes plus
// rank groups, which the plugin can emit cleanly without the user
// having to spell out the invisible-spacer dance themselves.

package graphviz

import (
	"fmt"
	"strings"

	"oss.terrastruct.com/d2/d2graph"
)

// pluginClusterKeys lists the cluster-attr keys consumed by the plugin
// (NOT forwarded to dot). Each maps to a private mechanism (spacer
// nodes, rank groups, etc.) inside this file.
var pluginClusterKeys = map[string]bool{
	"padding_top":       true,
	"padding_bottom":    true,
	"align_top_with":    true,
	"align_bottom_with": true,
	// rank_same_anchor controls which leaf of each child the plugin
	// uses when emitting the rank=same hint for a `direction: right`
	// or `direction: up` container. Values:
	//   "first" (default): use the first leaf of each child. Equivalent
	//                      to pinning children's tops together.
	//   "last":            use the last leaf of each child. Pins
	//                      bottoms together — useful when child columns
	//                      differ in length and you want the trailing
	//                      content aligned (e.g. main flow + side
	//                      cross-cutting cluster at bottom-right).
	//   "none":            suppress rank=same entirely. Graphviz lays
	//                      out children without per-rank pinning.
	"rank_same_anchor": true,
}

// alignRequest captures a cross-cluster rank-anchor request collected
// during writeObject traversal. Emitted as `{rank=same; ...}` after
// all clusters have been written.
type alignRequest struct {
	srcLeaf    string // AbsID of the first or last leaf of the source cluster
	targetLeaf string // AbsID of the first or last leaf of the target cluster
}

// stripPluginClusterKeys removes plugin-only keys from attrs and
// returns them in a separate map. attrs is mutated in place.
func stripPluginClusterKeys(attrs map[string]string) map[string]string {
	plugin := make(map[string]string)
	for k := range pluginClusterKeys {
		if v, ok := attrs[k]; ok {
			plugin[k] = v
			delete(attrs, k)
		}
	}
	return plugin
}

// emitTopPaddingSpacer writes an invisible zero-width node with the
// configured height plus a high-weight invisible constraint edge to
// the cluster's first leaf. dot's rank algorithm then places the
// spacer one rank above the first leaf, and the cluster bounding box
// extends to include it — giving the cluster `padding_top` units of
// visible space between its top border and its first visible content.
func emitTopPaddingSpacer(sb *strings.Builder, o *d2graph.Object, heightInches string, indent string) {
	spacerID := absID(o) + "__padtop"
	firstLeafID := absID(firstLeaf(o))
	fmt.Fprintf(sb, "%s%s [width=0.01, height=%s, style=invis, label=\"\"];\n",
		indent, dotQuote(spacerID), heightInches)
	fmt.Fprintf(sb, "%s%s -> %s [style=invis, weight=1000];\n",
		indent, dotQuote(spacerID), dotQuote(firstLeafID))
}

// emitBottomPaddingSpacer mirrors emitTopPaddingSpacer at the bottom.
// The invisible edge runs from the cluster's last leaf to the spacer,
// pushing the cluster's bounding box downward.
func emitBottomPaddingSpacer(sb *strings.Builder, o *d2graph.Object, heightInches string, indent string) {
	spacerID := absID(o) + "__padbot"
	lastLeafID := absID(lastLeaf(o))
	fmt.Fprintf(sb, "%s%s [width=0.01, height=%s, style=invis, label=\"\"];\n",
		indent, dotQuote(spacerID), heightInches)
	fmt.Fprintf(sb, "%s%s -> %s [style=invis, weight=1000];\n",
		indent, dotQuote(lastLeafID), dotQuote(spacerID))
}

// lastLeaf walks into the last child in D2 source order until it
// reaches a leaf. Mirrors firstLeaf for the bottom-side spacer wiring.
func lastLeaf(o *d2graph.Object) *d2graph.Object {
	cur := o
	for len(cur.ChildrenArray) > 0 {
		cur = cur.ChildrenArray[len(cur.ChildrenArray)-1]
	}
	return cur
}

// resolveAlignTarget turns an align_*_with cluster AbsID into the
// AbsID of a leaf node at the desired rank inside the target cluster.
// We always pick the target's last leaf, regardless of side, so the
// invisible edge originates from the side of the target that is
// closest to the source cluster (which sits to the right of the main
// flow in typical "side-cluster" layouts). The leaf still shares its
// rank with the rest of the cluster's same-rank operations, so the
// rank constraint we want is satisfied without pulling the target's
// internal layout sideways.
func resolveAlignTarget(o *d2graph.Object, targetClusterID string, side string) string {
	target := lookupObject(o, targetClusterID)
	if target == nil {
		// Caller wrote a bad AbsID — return the raw string and let dot
		// emit a warning rather than silently dropping the request.
		return targetClusterID
	}
	return absID(lastLeaf(target))
}

// lookupObject walks the graph from the root looking for an object
// whose AbsID matches the requested string. Returns nil if not found.
func lookupObject(any *d2graph.Object, id string) *d2graph.Object {
	root := any
	for root.Parent != nil {
		root = root.Parent
	}
	return walkForID(root, id)
}

func walkForID(o *d2graph.Object, id string) *d2graph.Object {
	if absID(o) == id {
		return o
	}
	for _, c := range o.ChildrenArray {
		if got := walkForID(c, id); got != nil {
			return got
		}
	}
	return nil
}
