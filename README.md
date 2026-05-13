# d2plugin-graphviz

D2 source, Graphviz layout, D2 rendering.

D2's dagre and ELK engines get tricky once you have clusters, back-edges, or things that need to line up across cluster boundaries. The diagrams come out stretched, the back-edges get drawn through node labels, and `rank=same` doesn't propagate where you want it. Graphviz `dot` handles those layouts pretty well, it just renders ugly compared to D2.

This plugin uses each tool for what it's good at. D2 parses your `.d2` source as usual. The plugin converts the parsed graph to a `.dot` file, shells out to `dot -Tjson` for positions and edge routes, and feeds the positioned graph back to D2's SVG renderer. You get D2's fonts/colours/rounded shapes with graphviz's layout control.

## Install

```sh
go install github.com/andrewleech/d2plugin-graphviz/cmd/d2plugin-graphviz@latest
```

That drops a `d2plugin-graphviz` binary into `$GOBIN` (default `$GOPATH/bin`). D2 discovers plugins by name in `$PATH`, so that directory has to be on `$PATH`.

You also need the graphviz `dot` binary:

```sh
# macOS
brew install graphviz

# Debian / Ubuntu
apt install graphviz
```

## Use

```sh
d2 --layout graphviz input.d2 output.svg
```

or set `D2_LAYOUT=graphviz` in the environment. `d2 --layout graphviz --help` lists the plugin's flags.

## What you get out of the box

A bunch of defaults are baked in so a fresh diagram with no config looks sensible:

- `compound=true`, `newrank=true` so rank constraints work across cluster boundaries.
- `nodesep=2.5`, `ranksep=2.5` so cluster boundaries don't touch when the per-cluster margin (below) is on. The graphviz defaults of 0.25 / 0.5 are too tight for any diagram with multiple visible clusters.
- Per visible cluster: `margin="40,30"` (asymmetric, 40pt horizontal, 30pt vertical), `labelloc=t`, `labeljust=l`. Gives the cluster label visible space above its contents and matches typical figure-caption convention.
- D2 root `direction` maps to graphviz `rankdir` automatically (`down` → `TB`, `right` → `LR`, etc).
- Invisible D2 edges (`style.opacity: 0`) come through as `[style=invis]` in dot, so any stacking tricks you already use (e.g. `a -> b: {style.opacity: 0}` to force ordering) still work.
- Transparent containers (`style.fill: transparent` and `style.stroke: transparent`) become `style=invis` subgraph clusters. They carry layout semantics without drawing a border, which is what you want for pure grouping.
- Cross-container edges get `compound=true, ltail=..., lhead=...` so they anchor to the cluster boundary rather than a leaf node.

All defaults can be overridden, see Configuration.

## Plugin-only cluster attributes

The plugin adds a few cluster-attr keys that aren't dot attributes; the plugin consumes them and emits the equivalent dot structures (invisible spacer nodes, rank groups, etc).

| Key | Purpose |
| --- | --- |
| `padding_top` | Invisible vertical space above the cluster's first leaf, in inches. Implemented as a zero-width invisible node + high-weight constraint edge to the first leaf. Be careful with large `ranksep` though, each extra invisible node consumes a full rank gap. |
| `padding_bottom` | Mirror of `padding_top` on the last leaf. |
| `align_top_with` | AbsID of another cluster. Emits a top-level `{rank=same; src_first_leaf; target_last_leaf}` that pins this cluster's top against the target. Useful for placing a side cluster (e.g. an error-handler column) at a specific row of the main flow column. |
| `align_bottom_with` | Mirror on the last leaf of this cluster. |
| `rank_same_anchor` | Override which leaf of each child is used when the plugin auto-emits `{rank=same; ...}` for `direction: right` (or `direction: up`) containers. `"first"` (default) pins tops. `"last"` pins bottoms; useful when child columns have different lengths and you want the trailing content aligned. `"none"` suppresses the hint entirely. |

Each goes in `cluster_attrs` under the cluster's AbsID, same as any other override:

```d2
vars: {
  d2-config: {
    data: {
      graphviz: "{\"cluster_attrs\": {\"root.errors\": {\"align_top_with\": \"root.flow.signal_ext\"}}}"
    }
  }
}
```

## Configuration

There are four layers, evaluated highest-wins:

1. **Plugin defaults** (above).
2. **CLI flags** for graph-wide attrs. Useful when you don't want config in the source.
3. **`vars.d2-config.data.graphviz`** in the source. JSON-encoded string, gives per-node / per-edge / per-cluster overrides plus rank groups.
4. **`classes: [...]`** in the source. Per-element shortcuts that map to bundled or user-defined attribute sets.

### CLI flags

All prefixed `--graphviz-`. Full list in `d2 --layout graphviz --help`. The common ones:

| Flag | Graphviz attr | Notes |
| --- | --- | --- |
| `--graphviz-rankdir` | `rankdir` | `TB`, `BT`, `LR`, `RL`. Empty derives from root `direction`. |
| `--graphviz-nodesep` | `nodesep` | Inches between siblings at the same rank. |
| `--graphviz-ranksep` | `ranksep` | Inches between ranks. |
| `--graphviz-splines` | `splines` | `true`, `spline`, `polyline`, `ortho`, `curved`, `false`. |
| `--graphviz-size` | `size` | Canvas bound, e.g. `7.3,10.2` for A4 portrait. |
| `--graphviz-ratio` | `ratio` | `fill`, `compress`, `expand`, `auto`, or a number. |
| `--graphviz-newrank` | `newrank` | `true` / `false`. Default `true`. |

### vars.d2-config.data.graphviz

D2's config parser (as of v0.7.1) only preserves flat primitives inside `vars.d2-config.data`, nested maps get dropped. The workaround is to encode the config as a JSON string:

```d2
vars: {
  d2-config: {
    data: {
      graphviz: "{
        \"global_attrs\": {
          \"rankdir\": \"TB\",
          \"nodesep\": \"0.25\",
          \"ranksep\": \"0.3\",
          \"splines\": \"spline\"
        },
        \"cluster_attrs\": {
          \"main.use\": {\"penwidth\": \"1.5\"}
        },
        \"edge_attrs\": {
          \"main.use.test_complete -> main.use.self_test\": {\"constraint\": \"false\"}
        },
        \"rank_groups\": [
          [\"main.right_col.svc\", \"main.right_col.ship\"]
        ]
      }"
    }
  }
}
```

Schema:

| Field | Type | Purpose |
| --- | --- | --- |
| `global_attrs` | `map<string,string>` | Merged into the graph-level `graph [...]` block. Overrides CLI flags. |
| `rank_groups` | `[[string]]` | Each inner list becomes `{rank=same; ...}` at the top level. Keys are D2 AbsIDs. |
| `node_attrs` | `map<AbsID, map<string,string>>` | Per-leaf node attribute overrides. |
| `edge_attrs` | `map<key, map<string,string>>` | Per-edge overrides. Key is `"<SrcAbsID> -> <DstAbsID>"` for the first match of that pair, or `"(Src -> Dst)[index]"` for parallel edges. |
| `cluster_attrs` | `map<AbsID, map<string,string>>` | Per-container overrides. Recognises the plugin-only keys above plus any raw dot attr. |
| `classes` | `map<class-name, map<string,string>>` | User-defined class-name shortcuts; see Layer 4. |

AbsID is the D2 dotted path (e.g. `main.use.ready`). To see what AbsIDs D2 generates for your input, dump the emitted dot:

```sh
DUMP_DOT_INPUT=path/to/input.d2 go test -run TestDumpDot -v ./internal/graphviz/
```

### Class shortcuts

Declare a class at root scope with `classes: { ... }`, attach it to an edge or node with `class: name` (singular), and the plugin will match the name against these built-ins:

| Class | Effect |
| --- | --- |
| `no-constraint` | `constraint=false`. Excludes the edge from rank assignment. Use on back-edges and recovery transitions so they don't stretch the layout. |
| `bold-path` | `style=bold, penwidth=2, weight=10`. Emphasised happy-path edge with layout pressure to stay straight. |
| `rank-same` | `group=rank-same`. Graphviz grouping hint. For true cross-cluster alignment use `rank_groups` instead. |

User-defined classes in `classes` (the schema field above) override built-ins of the same name.

Note on syntax: D2 v0.7.1 reserves `classes:` (plural) for the root-scope declaration and `class:` (singular) for the per-element reference. Putting `classes: [name]` on an edge will trigger `classes must be declared at a board root scope`.

```d2
classes: {
  no-constraint: {style.stroke-dash: 4}  # D2-side styling is optional
}
foo -> bar: {class: no-constraint}
```

The graphviz-side effects (e.g. `constraint=false`) are applied even when the class body is empty.

## When to reach for it

- Multiple clusters that need to align along a particular row or column.
- Back-edges and error transitions that you don't want pulling the main flow around.
- State machines, pipeline graphs, anything with retry loops or recovery branches.

If your graph is a simple tree or single flow with no back-edges, dagre is probably fine and you don't need this.

## Examples

Each subdirectory under [`examples/`](examples/) contains a focused diagram source plus rendered output for both engines. Re-render after edits with `scripts/render_examples.sh`.

### State machine with retries — [`examples/state_machine/`](examples/state_machine/)

Door access controller with retry, lockout, and sensor-fault transitions. The back-edges all converge on `LOCKED` — dagre routes them up the narrow right margin and stacks their labels; graphviz fans them out.

<table>
<tr><th>dagre</th><th>graphviz</th></tr>
<tr>
<td><img src="examples/state_machine/state_machine.dagre.svg" alt="dagre layout" width="420"></td>
<td><img src="examples/state_machine/state_machine.graphviz.svg" alt="graphviz layout" width="420"></td>
</tr>
</table>

### Pipeline with side-cluster failure handling — [`examples/pipeline/`](examples/pipeline/)

Build/test pipeline plus a failure-response cluster. The parent container sets `direction: right` so the two clusters should sit side-by-side. Dagre ignores the parent direction and stacks them; graphviz honours it and lets the failure-edge fan route cleanly across.

<table>
<tr><th>dagre</th><th>graphviz</th></tr>
<tr>
<td><img src="examples/pipeline/pipeline.dagre.svg" alt="dagre layout" width="420"></td>
<td><img src="examples/pipeline/pipeline.graphviz.svg" alt="graphviz layout" width="420"></td>
</tr>
</table>

### Class shortcuts on back-edges — [`examples/class_shortcuts/`](examples/class_shortcuts/)

Job worker with three back-edges into `READY` (lock lost, ack-next-job, retry-after-backoff). In the baseline graphviz render those back-edges share rank-assignment pressure with the forward edges; the label clutter at the top reflects that. The tuned variant marks them `class: no-constraint` (drops them from rank assignment) and the happy-path edges `class: bold-path` (`weight=10` to keep them straight, plus D2 styling for visual emphasis).

<table>
<tr><th>graphviz (baseline)</th><th>graphviz (tuned)</th></tr>
<tr>
<td><img src="examples/class_shortcuts/job_worker.graphviz.svg" alt="baseline" width="420"></td>
<td><img src="examples/class_shortcuts/job_worker.tuned.svg" alt="tuned with classes" width="420"></td>
</tr>
</table>

Note: D2 v0.7.1 wants the attribute on edges to be `class: name` (singular), even though the root-scope declaration is `classes: { ... }`. Mixing `classes: [name]` on an edge currently triggers `classes must be declared at a board root scope`.

### Cross-cluster row alignment — [`examples/cross_align/`](examples/cross_align/)

Primary service column with a parallel audit-trail cluster. Each service step writes a corresponding audit entry — but with seven service steps and only three audit entries, dot's edge-length minimisation parks the audit nodes at convenient ranks, not the rows you'd expect. `rank_groups` in the config pins each audit row to the service step that writes it.

<table>
<tr><th>graphviz (baseline)</th><th>graphviz (tuned)</th></tr>
<tr>
<td><img src="examples/cross_align/request_flow.graphviz.svg" alt="baseline" width="420"></td>
<td><img src="examples/cross_align/request_flow.tuned.svg" alt="tuned with rank_groups" width="420"></td>
</tr>
</table>

## Limitations

- Per-cluster `rankdir` is a graphviz limitation, not ours. The plugin approximates it with rank hints; complex cases need explicit `rank_groups`.
- Edge labels aren't sent to graphviz with their measured dimensions. The labels render correctly but dot doesn't reserve space for them, so dense graphs can get edge-label collisions. Coarser than dagre's edge-label avoidance.
- `padding_top` / `padding_bottom` add a whole invisible rank each. If your `ranksep` is large (2-3 inches), even a small padding value adds a full ranksep gap to the cluster height. Use raw `margin: "60"` (scalar inches) as a cluster attr if you just want more inside padding, not an extra rank.
- D2 grid and sequence diagrams are pre-laid-out by D2 before the plugin runs. They arrive as opaque pre-sized boxes; this plugin can't reshape them.
- Text / markdown / code shapes use their measured width as-is. A long unbroken `shape: text` line will grow the canvas. Wrap or use `--graphviz-size` to constrain.

## Development

```sh
go build ./...
go test ./...

# Rebuild, install, render examples/input.d2 with both dagre and graphviz,
# produces out/compare.html for a visual diff:
scripts/eval.sh
```

The `TestDumpDot` test dumps the dot source for any D2 file:

```sh
DUMP_DOT_INPUT=examples/input.d2 go test -run TestDumpDot -v ./internal/graphviz/
```

Handy when something's not laying out the way you'd expect and you want to see what the plugin is telling dot.

## Licence

Same as the project repo. The plugin depends on `oss.terrastruct.com/d2` (MPL-2.0 at time of writing).
