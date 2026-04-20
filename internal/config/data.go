package config

// DataConfig is the shape of the per-graph configuration channel read
// from `vars.d2-config.data.graphviz.*` in the D2 source. Everything
// here is optional; presence of a field overrides the corresponding
// CLI default.
//
// Keys for NodeAttrs / ClusterAttrs are D2 AbsIDs (e.g. "main.use.ready").
// EdgeAttrs keys can be either:
//   - the edge's dotted identifier as shown in D2 (e.g.
//     "(main.use.ready -> main.use.reading_cassette)[0]" — zero-indexed
//     per-pair), or
//   - "<SrcAbsID> -> <DstAbsID>" as a shorthand matching the first
//     occurrence of that Src/Dst pair.
type DataConfig struct {
	// GlobalAttrs are merged into the graph-level `graph [...]` block.
	// Later entries override earlier ones. Typical keys: rankdir,
	// nodesep, ranksep, splines, size, ratio, dpi, margin, pad.
	GlobalAttrs map[string]string `json:"global_attrs,omitempty"`

	// NodeAttrs applies to leaf nodes.
	NodeAttrs map[string]map[string]string `json:"node_attrs,omitempty"`

	// EdgeAttrs applies to edges.
	EdgeAttrs map[string]map[string]string `json:"edge_attrs,omitempty"`

	// ClusterAttrs applies to subgraph clusters (containers).
	ClusterAttrs map[string]map[string]string `json:"cluster_attrs,omitempty"`

	// RankGroups forces one or more `{rank=same; ...}` subgraphs at
	// the top level. Each inner slice lists AbsIDs to pin together.
	RankGroups [][]string `json:"rank_groups,omitempty"`

	// Classes maps D2 class names to Graphviz attribute bundles. When
	// a node/edge has `classes: [name]` in D2, those attrs are merged
	// in. Built-ins: "rank-same", "no-constraint", "bold-path".
	// User-declared entries override built-ins.
	Classes map[string]map[string]string `json:"classes,omitempty"`
}

// BuiltinClasses are the class-name shortcuts the plugin recognises by
// default. Applied to nodes and edges whose D2 `classes` list contains
// the key. A user-supplied Classes entry with the same key overrides.
var BuiltinClasses = map[string]map[string]string{
	// Aligns a node with others that share this class. Users should
	// still declare a rank_groups entry for cross-cluster grouping —
	// this class alone only hints to Graphviz via `group`.
	"rank-same": {"group": "rank-same"},

	// Removes the edge from rank assignment — useful for back-edges
	// and recovery transitions. Matches the single biggest win from
	// the pain list.
	"no-constraint": {"constraint": "false"},

	// Emphasised edges on the happy path.
	"bold-path": {"style": "bold", "penwidth": "2", "weight": "10"},
}

// ParseDataConfig extracts the graphviz section from g.Data. Returns
// an empty DataConfig if the section is missing or malformed; the
// plugin continues without overrides rather than failing layout.
//
// D2's `vars.d2-config.data` parser (d2compiler/compile.go ~L1566)
// only preserves top-level primitives and arrays of primitives —
// nested maps are silently dropped. To pass a rich configuration a
// user must supply a JSON-encoded string:
//
//	vars: {
//	  d2-config: {
//	    data: {
//	      graphviz: "{\"global_attrs\": {\"rankdir\": \"LR\"}}"
//	    }
//	  }
//	}
//
// We also keep the `graphviz: <nested map>` path working in case D2
// adds nested-map support upstream in a future release.
func ParseDataConfig(gData map[string]interface{}) DataConfig {
	var d DataConfig
	if gData == nil {
		return d
	}
	raw, ok := gData["graphviz"]
	if !ok {
		return d
	}
	switch v := raw.(type) {
	case string:
		// Empty or malformed JSON silently falls back to empty config
		// so the user still gets a rendered graph.
		if v != "" {
			parseJSONString(v, &d)
		}
	case map[string]interface{}:
		reencode(v, &d)
	}
	return d
}
