# d2-layout-graphviz: D2 Layout Plugin Using Graphviz

## Problem

D2 produces visually clean diagrams (good colour coding, containers, fonts, rounded boxes) but its bundled layout engines (dagre, ELK) have limited control over node placement. For diagrams with grouped states, back-edges, and cross-group connections, dagre stretches the layout and routes edges through other nodes. There's no way to constrain rank ordering, force alignment, or control edge routing.

Graphviz (dot) has excellent layout control: `rankdir`, `rank=same`, `minlen`, `weight`, subgraph ordering, and edge routing that avoids nodes. But its visual output is plain compared to D2.

## Goal

Build a D2 layout plugin (`d2-layout-graphviz`) that uses Graphviz's dot engine to compute node positions and edge routes, then feeds those positions back to D2 for rendering. This gives D2's visual style with Graphviz's layout control.

## How D2 Layout Plugins Work

D2 layout plugins are standalone executables that D2 discovers in `$PATH`. The protocol:

1. D2 calls the plugin binary with a JSON representation of the graph on stdin
2. The plugin computes positions for all nodes and edge routes
3. The plugin writes the positioned graph JSON to stdout
4. D2 renders the positioned graph using its own SVG/PNG renderer

The plugin binary must be named `d2plugin-<name>` (e.g. `d2plugin-graphviz`). Users invoke it via `d2 --layout graphviz input.d2 output.png`.

See: https://d2lang.com/tour/extensions#layout-engines

## Architecture

```
D2 input (.d2)
    |
    v
D2 parser (built into d2)
    |
    v  (JSON graph on stdin)
d2plugin-graphviz
    |
    +-- Convert D2 graph JSON to Graphviz dot
    |     - Map D2 containers to Graphviz subgraph clusters
    |     - Map D2 edges to Graphviz edges
    |     - Preserve D2 node IDs for round-trip
    |     - Apply Graphviz layout hints (rankdir, rank constraints)
    |
    +-- Run `dot -Tjson` to compute layout
    |     - Get x,y positions for all nodes
    |     - Get edge routing points (splines)
    |
    +-- Map positions back to D2 graph JSON
    |     - Set x, y, width, height on each D2 node
    |     - Set edge route points on each D2 edge
    |
    v  (positioned JSON on stdout)
D2 renderer (built into d2)
    |
    v
Output (SVG/PNG)
```

## What We Want To Achieve

### Input

The file `examples/input.d2` is a real-world state machine diagram with:
- 4 container groups (Normal Use, Service & Production, Shipping Battery, Error States)
- ~20 visible nodes across the groups
- Conditional branches within Normal Use
- Cross-group edges (from READY to service/shipping states)
- A collapsed Error States box with category text
- A legend/notes container

### Current Output (dagre)

`examples/current_dagre_output.png` shows what dagre produces: stretched layout, Service & Production and Shipping Battery not vertically aligned, edges routing through boxes.

### Desired Output

`examples/desired_layout_reference.png` shows a Graphviz-rendered version of the same state machine (from a hand-crafted .dot file at `examples/graphviz_reference.dot`). This demonstrates:

- **Compact vertical flow** within the Normal Use group
- **Grouped clusters** with colour-coded backgrounds and visible labels
- **Rank constraints** keeping related states aligned horizontally
- **Edge routing** that avoids passing through node boxes
- **A4 portrait fit** (~1500px wide at 200 DPI)
- **Happy path** visually highlighted (bold/coloured edges)
- **Error states** collapsed into a single categorised box

The goal of the plugin is to get this kind of layout control while keeping D2's visual rendering.

### Key Layout Properties That Graphviz Provides

1. `rankdir=TB` — top-to-bottom flow
2. `subgraph cluster_X` — visual grouping with background colours
3. `rank=same` — force nodes to align horizontally
4. `minlen` on edges — control minimum distance between connected nodes
5. `weight` on edges — influence which edges the layout prioritises for straightness
6. `constraint=false` on edges — prevent an edge from affecting rank assignment (useful for back-edges)
7. Edge routing avoids nodes by default (spline routing)

## Implementation Language

The plugin binary can be written in any language. Go is the natural choice (D2 itself is Go, and the plugin protocol is straightforward JSON). Python would also work but would be slower to start.

## Files in This Repository

```
examples/
  input.d2                      # Real D2 input to test with
  current_dagre_output.png      # What dagre produces (the problem)
  desired_layout_reference.png  # What we want it to look like (Graphviz layout)
  graphviz_reference.dot        # The Graphviz dot source that produced the desired layout
PLAN.md                         # This file
```

## Next Steps

1. Research the exact D2 plugin JSON protocol (stdin/stdout format, node/edge schema)
2. Build a minimal plugin that reads the D2 JSON, converts to dot, runs Graphviz, maps positions back
3. Test with `examples/input.d2` and compare against `examples/desired_layout_reference.png`
4. Handle containers/clusters mapping (D2 containers → Graphviz subgraph clusters)
5. Handle edge routing (Graphviz splines → D2 edge route points)
6. Package as a standalone binary installable via `go install` or similar

## Status

Awaiting direction to begin implementation.
