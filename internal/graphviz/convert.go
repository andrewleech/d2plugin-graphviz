package graphviz

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"oss.terrastruct.com/d2/d2graph"

	"github.com/andrewleech/d2plugin-graphviz/internal/config"
)

// absID returns a stable unique identifier for an object. The root has
// empty AbsID, but the root never appears as a node in the dot source.
func absID(o *d2graph.Object) string { return o.AbsID() }

// dotQuote escapes a string for use in a quoted dot identifier. Graphviz
// requires backslash-escaping of " and \. Newlines become \n (dot-style).
func dotQuote(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", "")
	return `"` + r.Replace(s) + `"`
}

// clusterName builds the subgraph cluster name for a container. The
// "cluster_" prefix is required for dot to treat it as a visual cluster.
func clusterName(o *d2graph.Object) string {
	return "cluster_" + absID(o)
}

// rootDirectionToRankdir maps D2's root `direction` to Graphviz rankdir.
// Called only when the user hasn't overridden via --graphviz-rankdir.
func rootDirectionToRankdir(dir string) string {
	switch dir {
	case "down", "":
		return "TB"
	case "up":
		return "BT"
	case "right":
		return "LR"
	case "left":
		return "RL"
	default:
		return "TB"
	}
}

// isContainer returns true if this object has children and should map to
// a subgraph cluster rather than a node.
func isContainer(o *d2graph.Object) bool { return len(o.ChildrenArray) > 0 }

// pxToInches converts D2 pixel dimensions to Graphviz inches at 72 DPI.
// Graphviz stores width/height in inches regardless of dpi setting —
// dpi only affects the output coordinate scaling.
func pxToInches(px float64) float64 { return px / 72.0 }

// BuildDot serializes *d2graph.Graph into a Graphviz .dot source.
//
// Nodes and clusters carry the D2 AbsID as their identifier so the
// output of `dot -Tjson` can be mapped back one-to-one. Labels are
// emitted as empty strings with fixedsize=true width/height taken from
// the pre-measured d2graph boxes — Graphviz only needs the bounding
// box to lay out, and D2 will render labels with its own text engine.
func BuildDot(g *d2graph.Graph, opts config.CLIOpts) (string, error) {
	data := config.ParseDataConfig(g.Data)

	var sb strings.Builder

	// ----- graph-wide attrs -----
	// Precedence (highest wins): data.GlobalAttrs > CLI flags > plugin
	// defaults > graphviz defaults.
	//
	// compound=true is a hard requirement for ltail/lhead (cluster
	// boundary edge routing).
	//
	// nodesep and ranksep are bumped from graphviz's defaults (0.25 and
	// 0.5 inches) to values that pair sensibly with the default cluster
	// margin "40,30" emitted below. With margin=40 on each side of two
	// adjacent clusters, nodesep<0.8 inches produces zero or negative
	// visible space between cluster boundaries — defaulting to 1.0
	// gives ~15 px of inter-cluster horizontal gap. Similarly ranksep
	// 1.0 gives clear vertical breathing room between cluster labels
	// and adjacent cluster contents.
	//
	// newrank=true is enabled by default so rank constraints propagate
	// across cluster boundaries (necessary for align_top_with /
	// align_bottom_with to work).
	globalAttrs := map[string]string{
		"compound": "true",
		"nodesep":  "2.5",
		"ranksep":  "2.5",
		"newrank":  "true",
	}
	rankdir, ok := opts.Attr("rankdir")
	if !ok {
		rankdir = rootDirectionToRankdir(g.Root.Direction.Value)
	}
	globalAttrs["rankdir"] = rankdir
	for _, a := range []string{"nodesep", "ranksep", "splines", "newrank",
		"concentrate", "overlap", "margin", "pad", "ratio", "size", "dpi"} {
		if v, ok := opts.Attr(a); ok {
			globalAttrs[a] = v
		}
	}
	for k, v := range data.GlobalAttrs {
		globalAttrs[k] = v
	}

	sb.WriteString("digraph G {\n")
	sb.WriteString("  graph [")
	writeAttrListInline(&sb, globalAttrs)
	sb.WriteString("];\n")
	sb.WriteString("  node [shape=box, fixedsize=true, label=\"\"];\n")
	sb.WriteString("  edge [label=\"\"];\n\n")

	aligns := make([]alignRequest, 0)
	ctx := buildCtx{rankdir: globalAttrs["rankdir"], data: data, alignRequests: &aligns}

	// ----- node + cluster tree -----
	// Top-level children of root. Root itself isn't emitted.
	for _, child := range g.Root.ChildrenArray {
		writeObject(&sb, child, "  ", ctx)
	}

	// ----- edges -----
	for _, e := range g.Edges {
		writeEdge(&sb, e, ctx)
	}

	// ----- top-level rank groups from data config -----
	for _, grp := range data.RankGroups {
		if len(grp) < 2 {
			continue
		}
		quoted := make([]string, len(grp))
		for i, id := range grp {
			quoted[i] = dotQuote(id)
		}
		fmt.Fprintf(&sb, "  {rank=same; %s;}\n", strings.Join(quoted, "; "))
	}

	// ----- rank=same groups from align_top_with / align_bottom_with --
	//
	// Emit a top-level `{rank=same; ...}` group that pins the source
	// leaf and target leaf to the same rank. We deliberately do NOT
	// pair this with a high-weight invisible edge — dot's edge-length
	// minimisation would otherwise pull the source's cluster
	// horizontally toward the target's column.
	for _, req := range aligns {
		fmt.Fprintf(&sb, "  {rank=same; %s; %s;}\n",
			dotQuote(req.srcLeaf), dotQuote(req.targetLeaf))
	}

	sb.WriteString("}\n")
	return sb.String(), nil
}

// writeAttrListInline emits attrs as a comma-separated list inside a
// `[...]` attribute block. Stable key order. Empty values are skipped.
func writeAttrListInline(sb *strings.Builder, attrs map[string]string) {
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		if attrs[k] == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, k := range keys {
		if i > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprintf(sb, "%s=%s", k, dotValue(attrs[k]))
	}
}

// dotValue emits a Graphviz attribute value: quoted only when the
// value is a valid bare Graphviz identifier or numeral.
//
// Graphviz accepts unquoted:
//   - alphabetic strings `[a-zA-Z_\x80-\xff][a-zA-Z_0-9\x80-\xff]*`
//   - numerals `-? ( \.[0-9]+ | [0-9]+ (\.[0-9]*)? )`
//
// Dots in identifiers (e.g. `cluster_main.use`) are NOT allowed
// unquoted. We conservatively quote anything that isn't purely
// alphanumeric-underscore or a plain number.
func dotValue(v string) string {
	if v == "" {
		return `""`
	}
	if isBareID(v) || isBareNumeral(v) {
		return v
	}
	return dotQuote(v)
}

func isBareID(v string) bool {
	if v == "" {
		return false
	}
	for i, r := range v {
		isAlpha := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_'
		isDigit := r >= '0' && r <= '9'
		if i == 0 {
			if !isAlpha {
				return false
			}
		} else if !isAlpha && !isDigit {
			return false
		}
	}
	return true
}

func isBareNumeral(v string) bool {
	s := v
	if strings.HasPrefix(s, "-") {
		s = s[1:]
	}
	if s == "" {
		return false
	}
	seenDot := false
	seenDigit := false
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			seenDigit = true
		case r == '.':
			if seenDot {
				return false
			}
			seenDot = true
		default:
			return false
		}
	}
	return seenDigit
}

// buildCtx threads graph-wide state into the recursive writer.
type buildCtx struct {
	rankdir       string
	data          config.DataConfig
	alignRequests *[]alignRequest // collected during traversal, emitted at end of BuildDot
}

// classAttrs collects built-in and user-defined class-shortcut attrs
// for the given class list. User-defined classes override built-ins.
func (c buildCtx) classAttrs(classes []string) map[string]string {
	out := map[string]string{}
	for _, cls := range classes {
		if attrs, ok := config.BuiltinClasses[cls]; ok {
			for k, v := range attrs {
				out[k] = v
			}
		}
		if attrs, ok := c.data.Classes[cls]; ok {
			for k, v := range attrs {
				out[k] = v
			}
		}
	}
	return out
}

// writeObject emits a container (subgraph cluster) or a leaf (node).
// Recurses into children. Indentation is cosmetic only.
func writeObject(sb *strings.Builder, o *d2graph.Object, indent string, ctx buildCtx) {
	if isContainer(o) {
		fmt.Fprintf(sb, "%ssubgraph %s {\n", indent, dotQuote(clusterName(o)))
		// Base cluster attrs: label + invisibility hint. Always emit
		// label= (even empty) so dot doesn't fall back to the subgraph
		// identifier as the displayed label.
		cattrs := map[string]string{"label": o.Label.Value}
		if isInvisibleContainer(o) {
			cattrs["style"] = "invis"
		} else {
			// Defaults applied to every visible cluster so users don't
			// have to repeat the same margin / label-position attrs on
			// every cluster_attrs entry. Asymmetric margin (horiz, vert)
			// gives the cluster label visible space above the contents.
			// Top-left label position matches typical document figure
			// conventions. User overrides via ClusterAttrs win.
			cattrs["margin"] = "40,30"
			cattrs["labelloc"] = "t"
			cattrs["labeljust"] = "l"
		}
		// Merge data.ClusterAttrs (user overrides).
		for k, v := range ctx.data.ClusterAttrs[absID(o)] {
			cattrs[k] = v
		}
		// Extract plugin-only keys before forwarding the remaining
		// attrs to dot. These keys are not dot attributes; they trigger
		// spacer-node emission, rank-group registration, or per-cluster
		// rank-hint overrides by the plugin.
		pluginAttrs := stripPluginClusterKeys(cattrs)
		writeAttrListLines(sb, cattrs, indent+"  ")

		// Top padding via invisible spacer node + high-weight invis edge
		// to first leaf. dot's rank algorithm places the spacer one
		// rank above the first leaf, extending the cluster bounding
		// box upward.
		if pt := pluginAttrs["padding_top"]; pt != "" {
			emitTopPaddingSpacer(sb, o, pt, indent+"  ")
		}

		// Children.
		for _, c := range o.ChildrenArray {
			writeObject(sb, c, indent+"  ", ctx)
		}

		// Bottom padding mirror.
		if pb := pluginAttrs["padding_bottom"]; pb != "" {
			emitBottomPaddingSpacer(sb, o, pb, indent+"  ")
		}

		// Per-container direction hint. `rank_same_anchor` (plugin-only)
		// overrides the default behaviour of pinning the first leaves;
		// "last" pins last leaves, "none" suppresses the hint.
		anchor := pluginAttrs["rank_same_anchor"]
		if anchor != "none" {
			if hint := rankSameHintWithAnchor(o, ctx.rankdir, anchor); hint != "" {
				fmt.Fprintf(sb, "%s  %s\n", indent, hint)
			}
		}
		fmt.Fprintf(sb, "%s}\n", indent)

		// Cross-cluster rank-anchor requests. Registered now and
		// emitted as top-level `{rank=same; ...}` groups after all
		// clusters have been written (see BuildDot tail).
		if a := pluginAttrs["align_top_with"]; a != "" {
			*ctx.alignRequests = append(*ctx.alignRequests, alignRequest{
				srcLeaf:    absID(firstLeaf(o)),
				targetLeaf: resolveAlignTarget(o, a, "top"),
			})
		}
		if a := pluginAttrs["align_bottom_with"]; a != "" {
			*ctx.alignRequests = append(*ctx.alignRequests, alignRequest{
				srcLeaf:    absID(lastLeaf(o)),
				targetLeaf: resolveAlignTarget(o, a, "bottom"),
			})
		}
		return
	}
	// Leaf node.
	w, h := nodeSize(o)
	nattrs := map[string]string{
		"width":  fmt.Sprintf("%.4f", pxToInches(w)),
		"height": fmt.Sprintf("%.4f", pxToInches(h)),
	}
	// Apply class-based shortcuts then user overrides.
	for k, v := range ctx.classAttrs(o.Classes) {
		nattrs[k] = v
	}
	for k, v := range ctx.data.NodeAttrs[absID(o)] {
		nattrs[k] = v
	}
	fmt.Fprintf(sb, "%s%s [", indent, dotQuote(absID(o)))
	first := true
	keys := sortedKeys(nattrs)
	for _, k := range keys {
		if nattrs[k] == "" {
			continue
		}
		if !first {
			sb.WriteString(", ")
		}
		fmt.Fprintf(sb, "%s=%s", k, dotValue(nattrs[k]))
		first = false
	}
	sb.WriteString("];\n")
}

// writeAttrListLines emits cluster-scope attributes as one-per-line
// statements. Clusters can't use the [attr=val,...] syntax at their
// declaration; attributes appear as bare statements inside the braces.
// Emits attrs with empty values as `key=""` so that e.g. an explicit
// empty label overrides Graphviz's fallback to the cluster name.
func writeAttrListLines(sb *strings.Builder, attrs map[string]string, indent string) {
	for _, k := range sortedKeys(attrs) {
		fmt.Fprintf(sb, "%s%s=%s;\n", indent, k, dotValue(attrs[k]))
	}
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// rankSameHint returns a `{rank=same; a; b; c}` subgraph line if this
// container's direction is perpendicular to the active rankdir. That
// aligns the children in a perpendicular row/column — the layout
// effect a D2 user expects from `direction: right` inside a vertical
// diagram.
//
// Children that are themselves containers use a representative leaf
// (firstLeaf) because dot's rank hints apply to nodes, not subgraphs.
func rankSameHint(o *d2graph.Object, rankdir string) string {
	dir := o.Direction.Value
	if dir == "" {
		return ""
	}
	perpendicular := map[string]string{
		"TB": "right",
		"BT": "right",
		"LR": "down",
		"RL": "down",
	}
	if perpendicular[rankdir] != dir && !(rankdir == "TB" && dir == "left") &&
		!(rankdir == "BT" && dir == "left") &&
		!(rankdir == "LR" && dir == "up") &&
		!(rankdir == "RL" && dir == "up") {
		return ""
	}
	var ids []string
	for _, c := range o.ChildrenArray {
		var leaf *d2graph.Object
		if isContainer(c) {
			leaf = firstLeaf(c)
		} else {
			leaf = c
		}
		ids = append(ids, dotQuote(absID(leaf)))
	}
	if len(ids) < 2 {
		return ""
	}
	return "{rank=same; " + strings.Join(ids, "; ") + ";}"
}

// rankSameHintWithAnchor is rankSameHint with control over which leaf
// of each child is used. anchor == "last" uses the last leaf; "" or
// any other value uses the first leaf (matches the legacy behaviour).
func rankSameHintWithAnchor(o *d2graph.Object, rankdir, anchor string) string {
	if anchor != "last" {
		return rankSameHint(o, rankdir)
	}
	dir := o.Direction.Value
	if dir == "" {
		return ""
	}
	perpendicular := map[string]string{
		"TB": "right",
		"BT": "right",
		"LR": "down",
		"RL": "down",
	}
	if perpendicular[rankdir] != dir && !(rankdir == "TB" && dir == "left") &&
		!(rankdir == "BT" && dir == "left") &&
		!(rankdir == "LR" && dir == "up") &&
		!(rankdir == "RL" && dir == "up") {
		return ""
	}
	var ids []string
	for _, c := range o.ChildrenArray {
		var leaf *d2graph.Object
		if isContainer(c) {
			leaf = lastLeaf(c)
		} else {
			leaf = c
		}
		ids = append(ids, dotQuote(absID(leaf)))
	}
	if len(ids) < 2 {
		return ""
	}
	return "{rank=same; " + strings.Join(ids, "; ") + ";}"
}

// isInvisibleContainer detects containers the user intends as pure
// layout groupings: transparent fill AND transparent stroke. D2
// accepts both "transparent" and the hex "#00000000" as transparent
// values; we match both. Style.Opacity at "0" also counts.
func isInvisibleContainer(o *d2graph.Object) bool {
	isTransparent := func(v string) bool {
		v = strings.ToLower(strings.TrimSpace(v))
		return v == "transparent" || v == "#00000000" || v == "none"
	}
	s := o.Style
	fillTransparent := s.Fill != nil && isTransparent(s.Fill.Value)
	strokeTransparent := s.Stroke != nil && isTransparent(s.Stroke.Value)
	if fillTransparent && strokeTransparent {
		return true
	}
	if s.Opacity != nil && s.Opacity.Value == "0" {
		return true
	}
	return false
}

// nodeSize returns the pixel width/height that D2 has already computed
// for this object. Falls back to a small default if somehow missing —
// this shouldn't happen for real graphs but guards against nil Box.
func nodeSize(o *d2graph.Object) (float64, float64) {
	if o.Box == nil {
		return 100, 60
	}
	w, h := o.Width, o.Height
	if w <= 0 {
		w = 100
	}
	if h <= 0 {
		h = 60
	}
	return w, h
}

// writeEdge emits a single edge. For edges whose endpoint is a
// container, dot doesn't natively support cluster-as-endpoint; we use
// compound=true + ltail/lhead + a representative descendant leaf.
func writeEdge(sb *strings.Builder, e *d2graph.Edge, ctx buildCtx) {
	src, srcLtail := endpoint(e.Src)
	dst, dstLhead := endpoint(e.Dst)
	attrs := map[string]string{}
	if srcLtail != "" {
		attrs["ltail"] = srcLtail
	}
	if dstLhead != "" {
		attrs["lhead"] = dstLhead
	}
	// D2 invisible edges (user's stacking hints) → dot style=invis.
	if e.Style.Opacity != nil && e.Style.Opacity.Value == "0" {
		attrs["style"] = "invis"
	}
	// Class-based shortcuts.
	edgeClasses := e.Classes
	for k, v := range ctx.classAttrs(edgeClasses) {
		attrs[k] = v
	}
	// Per-edge user overrides from data config. Match on either the
	// shorthand "Src -> Dst" or the per-pair indexed form.
	if o, ok := ctx.data.EdgeAttrs[edgeKey(e)]; ok {
		for k, v := range o {
			attrs[k] = v
		}
	}
	if o, ok := ctx.data.EdgeAttrs[edgeShortKey(e)]; ok {
		for k, v := range o {
			attrs[k] = v
		}
	}
	fmt.Fprintf(sb, "  %s -> %s", dotQuote(src), dotQuote(dst))
	if len(attrs) > 0 {
		fmt.Fprintf(sb, " [")
		first := true
		for _, k := range sortedKeys(attrs) {
			if attrs[k] == "" {
				continue
			}
			if !first {
				sb.WriteString(", ")
			}
			fmt.Fprintf(sb, "%s=%s", k, dotValue(attrs[k]))
			first = false
		}
		sb.WriteString("]")
	}
	sb.WriteString(";\n")
}

// edgeShortKey returns the "Src -> Dst" shorthand used for edge attr
// lookups in the data config.
func edgeShortKey(e *d2graph.Edge) string {
	return absID(e.Src) + " -> " + absID(e.Dst)
}

// edgeKey returns the fully disambiguated form "(Src -> Dst)[index]"
// used when multiple parallel edges share the same endpoints.
func edgeKey(e *d2graph.Edge) string {
	return fmt.Sprintf("(%s -> %s)[%d]", absID(e.Src), absID(e.Dst), e.Index)
}

// endpoint returns the dot-level endpoint for an edge end. If the D2
// endpoint is a container, returns a representative descendant leaf
// plus the cluster name for ltail/lhead.
func endpoint(o *d2graph.Object) (string, string) {
	if !isContainer(o) {
		return absID(o), ""
	}
	return absID(firstLeaf(o)), clusterName(o)
}

// firstLeaf walks into the first child in D2 source order until it
// reaches a leaf. D2 guarantees a container has at least one child.
// Source order is the natural "entry point" users expect (e.g. the
// first-declared state in a cluster).
func firstLeaf(o *d2graph.Object) *d2graph.Object {
	for isContainer(o) {
		o = o.ChildrenArray[0]
	}
	return o
}

// _ silences unused-import warnings during scaffolding.
var _ = math.Round
var _ = sort.Slice
