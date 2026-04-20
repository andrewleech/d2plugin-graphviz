# d2plugin-graphviz

A D2 layout plugin that uses Graphviz `dot` for layout while keeping
D2's SVG rendering. Designed for diagrams where D2's bundled dagre and
ELK engines produce stretched or cluster-misaligned output — regulated
software documentation (IEC 62304 SDS, SAD, ICD), state-machine
reference diagrams, and the like.

## What it does

1. D2 parses `.d2` source and produces a graph JSON.
2. This plugin translates that graph into Graphviz `.dot`, shells out
   to `dot -Tjson`, and maps positions and spline edge routes back.
3. D2 renders the positioned graph with its own SVG renderer — fonts,
   colours, icons, rounded-corner shapes, etc.

You get Graphviz's layout control (`rank=same`, `constraint=false`,
`minlen`, `weight`, subgraph clusters, spline routing that avoids
nodes) with D2's visual style.

## Install

```sh
go install github.com/andrewleech/d2plugin-graphviz/cmd/d2plugin-graphviz@latest
```

This drops a `d2plugin-graphviz` binary into `$GOBIN` (default
`$GOPATH/bin`). D2 discovers layout plugins named `d2plugin-<name>` in
`$PATH`, so that directory must be on `$PATH`.

Runtime requirement: the Graphviz `dot` binary must be in `$PATH`.

```sh
# macOS
brew install graphviz

# Debian/Ubuntu
apt install graphviz
```

## Use

```sh
d2 --layout graphviz input.d2 output.svg
```

`d2 --help` will list the plugin's flags once it's discovered.

## Configuration

There are four layers, evaluated in this precedence (highest wins):

1. **Defaults baked into the plugin** — sensible choices for a simple
   graph. `splines=true`, `newrank=true`, `rankdir` derived from the
   D2 root `direction`, invisible edges and transparent containers
   mapped to `[style=invis]`.
2. **CLI flags** — graph-wide Graphviz attributes.
3. **`vars.d2-config.data.graphviz` in the D2 source** — full
   expressivity including per-node, per-edge, per-cluster overrides
   and rank groups.
4. **`classes: [...]`** in the D2 source — per-element shortcuts.

### Layer 2: CLI flags

All flags are prefixed `--graphviz-`. Full list in `d2 --layout graphviz --help`:

| Flag | Maps to Graphviz | Notes |
| --- | --- | --- |
| `--graphviz-rankdir` | `rankdir` | `TB`, `BT`, `LR`, `RL`. Empty = derive from root `direction`. |
| `--graphviz-nodesep` | `nodesep` | Inches between sibling nodes. |
| `--graphviz-ranksep` | `ranksep` | Inches between ranks. |
| `--graphviz-splines` | `splines` | `true`, `polyline`, `ortho`, `spline`, `curved`, `false`. |
| `--graphviz-size` | `size` | Canvas size, e.g. `7.3,10.2` for A4 portrait. |
| `--graphviz-ratio` | `ratio` | `fill`, `compress`, `expand`, `auto`, or a number. |
| `--graphviz-dpi` | `dpi` | Output DPI. Default 72 (matches D2's pixel space 1:1). |
| `--graphviz-newrank` | `newrank` | `true` / `false`. |
| `--graphviz-concentrate` | `concentrate` | Merge parallel edges. |
| `--graphviz-overlap` | `overlap` | Overlap resolution (mostly for non-dot engines). |
| `--graphviz-margin` | `margin` | Graph-wide margin. |
| `--graphviz-pad` | `pad` | Graph-wide pad. |

Example: compact A4 portrait fit at 200 DPI:

```sh
d2 --layout graphviz \
   --graphviz-size "7.3,10.2" \
   --graphviz-ratio compress \
   --graphviz-dpi 200 \
   --graphviz-nodesep 0.2 \
   --graphviz-ranksep 0.3 \
   input.d2 output.png
```

### Layer 3: `vars.d2-config.data.graphviz`

D2's config parser (as of v0.7.1) preserves only flat primitives and
arrays of primitives inside `vars.d2-config.data`. To pass a rich
graphviz configuration, encode it as a **JSON string**:

```d2
vars: {
  d2-config: {
    data: {
      graphviz: "{
        \"global_attrs\": {
          \"rankdir\": \"TB\",
          \"nodesep\": \"0.25\",
          \"ranksep\": \"0.3\",
          \"splines\": \"spline\",
          \"newrank\": \"true\"
        },
        \"rank_groups\": [
          [\"main.right_col.svc\", \"main.right_col.ship\"],
          [\"main.use.heating_cassette\", \"main.use.test_auth\"]
        ],
        \"cluster_attrs\": {
          \"main.use\": {\"margin\": \"8\", \"penwidth\": \"1.5\"}
        },
        \"node_attrs\": {
          \"main.use.ready\": {\"penwidth\": \"2\"}
        },
        \"edge_attrs\": {
          \"main.use.test_complete -> main.use.self_test\": {\"constraint\": \"false\"},
          \"main.use.self_check_pass -> main.use.self_test\": {\"constraint\": \"false\"},
          \"main.use.init -> main.use.power_on\": {\"weight\": \"10\", \"penwidth\": \"2\"}
        }
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
| `edge_attrs` | `map<key, map<string,string>>` | Per-edge overrides. Key is `"<SrcAbsID> -> <DstAbsID>"` (first match of that pair) or `"(Src -> Dst)[index]"` for parallel edges. |
| `cluster_attrs` | `map<AbsID, map<string,string>>` | Per-container (subgraph cluster) overrides. |
| `classes` | `map<class-name, map<string,string>>` | User-defined class-name shortcuts; see Layer 4. |

AbsID is the D2-unique dotted path (e.g. `main.use.ready`). Use
`d2plugin-graphviz` stderr output or the `examples/input.d2` dump in
the repo's `internal/graphviz/dump_test.go` to see the AbsIDs D2
generates for your input.

### Layer 4: Class shortcuts

D2 already supports `classes: [name]` on nodes and edges. The plugin
recognises these built-in class names and applies the corresponding
Graphviz attrs:

| Class | Effect |
| --- | --- |
| `no-constraint` | `constraint=false` — excludes the edge from rank assignment. Use this on back-edges and recovery transitions to stop them stretching the layout. |
| `bold-path` | `style=bold, penwidth=2, weight=10` — emphasised happy-path edge with layout pressure to stay straight. |
| `rank-same` | `group=rank-same` — a Graphviz grouping hint. For true cross-cluster alignment use `rank_groups` in Layer 3. |

User-defined classes in Layer 3's `classes` map override built-ins
with the same key.

Example:

```d2
happy_path: {
  style.stroke-width: 3
}
ready -> reading_cassette: { classes: [bold-path] }
test_complete -> self_test: remove cassette { classes: [no-constraint] }
```

## Smart defaults (Layer 1)

The plugin translates several D2 conventions automatically without
any config:

- **Root `direction`** → `rankdir` (down → TB, right → LR, etc).
- **Invisible edges** (`style.opacity: 0` in D2) → `[style=invis]` in
  dot. Lets the existing stacking tricks (`a -> b: {style.opacity:0}`)
  continue to work.
- **Transparent containers** (both `style.fill: transparent` and
  `style.stroke: transparent`) → `style=invis` subgraph clusters.
  Carries layout semantics without drawing a border — useful for pure
  grouping.
- **Per-container `direction`** → when a container's `direction` is
  perpendicular to the effective rankdir, the plugin emits
  `{rank=same; <first-leaves>}` inside that cluster to approximate a
  lateral row/column. Graphviz does not support per-cluster `rankdir`
  natively; this is a best-effort approximation.
- **Cross-container edges** → `compound=true, ltail=..., lhead=...`
  so the edge is anchored at the cluster boundary.
- **Label positions** — populated with dagre-compatible defaults so
  D2's post-plugin renderer doesn't nil-deref.

## Limitations

- **Per-cluster `rankdir`** is a Graphviz limitation, not ours. The
  plugin approximates it with rank hints; complex cases may need
  explicit `rank_groups`.
- **Text / markdown / code shapes** use their full measured width.
  If a `shape: text` block has a very long line, the graph canvas
  will grow to fit. Wrap or truncate in your D2 source to control
  this, or use `--graphviz-size` to force a bounding box.
- **Edge labels** are not passed to Graphviz for layout spacing —
  dot doesn't receive label dimensions. Labels still render
  correctly but may overlap other edges in dense graphs.
- **Grid and sequence diagrams** are handled by D2 internally
  before reaching the plugin. They arrive as opaque pre-sized
  boxes.
- **Edge-label collision avoidance** is coarser than dagre's.

## Development

```sh
go build ./...
go test ./...

# Rebuild + install + render examples/input.d2 with both layouts,
# producing out/compare.html for visual diff:
scripts/eval.sh
```

The `TestDumpDot` test in `internal/graphviz/` emits the .dot source
the plugin would produce for any D2 file, useful for debugging:

```sh
DUMP_DOT_INPUT=examples/input.d2 go test -run TestDumpDot -v ./internal/graphviz/
```

## License

Same as the project repo. The plugin depends on `oss.terrastruct.com/d2`
which has its own licence (MPL-2.0 at time of writing).
