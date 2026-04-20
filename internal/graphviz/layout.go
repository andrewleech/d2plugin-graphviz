package graphviz

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os/exec"

	"oss.terrastruct.com/d2/d2graph"
	"oss.terrastruct.com/d2/lib/geo"
	"oss.terrastruct.com/d2/lib/label"

	"github.com/andrewleech/d2plugin-graphviz/internal/config"
)

// Layout computes positions for a D2 graph by shelling out to `dot` and
// mapping the result back onto g.Root / g.Objects / g.Edges.
func Layout(ctx context.Context, g *d2graph.Graph, opts config.CLIOpts) error {
	if err := checkDot(); err != nil {
		return err
	}

	// D2's post-plugin pipeline (d2near, DefaultRouter, exporter) reads
	// LabelPosition / IconPosition off objects and edges; if they're nil
	// on anything with a label, it panics. Dagre handles this with
	// positionLabelsIcons before routing; we mirror the same defaults.
	positionLabels(g)

	dotSrc, err := BuildDot(g, opts)
	if err != nil {
		return fmt.Errorf("build dot source: %w", err)
	}

	out, err := runDot(ctx, dotSrc)
	if err != nil {
		return err
	}

	return applyPositions(g, out)
}

// checkDot surfaces a clean error if `dot` is not installed. Called on
// every Layout invocation; exec.LookPath is cheap.
func checkDot() error {
	_, err := exec.LookPath("dot")
	if err != nil {
		return errors.New("d2plugin-graphviz requires the `dot` binary from Graphviz to be installed in $PATH. " +
			"Install the graphviz package for your platform (e.g. brew install graphviz, apt install graphviz).")
	}
	return nil
}

// applyPositions walks the dot output and writes positions back into
// the D2 graph. The conversion:
//
//   - Graphviz output is in points (pt) with origin at bottom-left,
//     y-axis up. D2 uses pixels with origin at top-left, y-axis down.
//   - Graphviz reports node `pos` as the centre; D2's `TopLeft`
//     wants the top-left corner.
//   - We flip y using the overall graph bounding-box height so that
//     the resulting picture starts at y=0.
//   - dpi isn't applied here: we keep points as pixels 1:1. The --dpi
//     flag is passed through to dot but doesn't change how dot reports
//     positions (pt is pt regardless of the dpi attribute).
func applyPositions(g *d2graph.Graph, out *dotOutput) error {
	_, _, _, gbY2, err := parseBB(out.BB)
	if err != nil {
		return fmt.Errorf("parse graph bb: %w", err)
	}
	flipY := func(y float64) float64 { return gbY2 - y }

	// Lookup by name (AbsID or cluster_<AbsID>).
	byName := map[string]dotObject{}
	for _, o := range out.Objects {
		byName[o.Name] = o
	}

	// ----- leaf + container node positions -----
	for _, obj := range g.Objects {
		var dot dotObject
		var ok bool
		if isContainer(obj) {
			dot, ok = byName[clusterName(obj)]
		} else {
			dot, ok = byName[absID(obj)]
		}
		if !ok {
			// Not emitted (e.g. grid/sequence pre-layouts) — leave as-is.
			continue
		}
		if isContainer(obj) {
			x1, y1, x2, y2, err := parseBB(dot.BB)
			if err != nil {
				return fmt.Errorf("parse cluster bb for %q: %w", absID(obj), err)
			}
			// Flip both y endpoints, then re-derive top/left.
			fy1 := flipY(y2) // cluster's visually-upper y after flip
			fy2 := flipY(y1)
			obj.Box = &geo.Box{
				TopLeft: geo.NewPoint(math.Round(x1), math.Round(fy1)),
				Width:   math.Round(x2 - x1),
				Height:  math.Round(fy2 - fy1),
			}
			continue
		}
		// Leaf.
		cx, cy, err := parsePoint(dot.Pos)
		if err != nil {
			return fmt.Errorf("parse node pos for %q: %w", absID(obj), err)
		}
		w, _ := parseFloat(dot.Width)
		h, _ := parseFloat(dot.Height)
		// Node width/height come back in inches. Convert to points (pt)
		// for 1:1 pixel equivalence at 72 dpi.
		wpt := w * 72.0
		hpt := h * 72.0
		fy := flipY(cy)
		obj.Box = &geo.Box{
			TopLeft: geo.NewPoint(math.Round(cx-wpt/2), math.Round(fy-hpt/2)),
			Width:   math.Round(wpt),
			Height:  math.Round(hpt),
		}
	}

	// ----- edge routes -----
	if len(out.Edges) != 0 {
		// Build an index: endpoint _gvid pair -> dotEdge. Multiple edges
		// between the same pair get matched by order of occurrence.
		type key struct{ tail, head int }
		buckets := map[key][]dotEdge{}
		for _, e := range out.Edges {
			k := key{e.Tail, e.Head}
			buckets[k] = append(buckets[k], e)
		}
		// Name -> _gvid for endpoint lookup.
		nameToGvid := map[string]int{}
		for _, o := range out.Objects {
			nameToGvid[o.Name] = o.Gvid
		}

		// Track consumption per bucket so parallel edges are taken in order.
		cursor := map[key]int{}
		for _, edge := range g.Edges {
			src, _ := endpoint(edge.Src)
			dst, _ := endpoint(edge.Dst)
			k := key{nameToGvid[src], nameToGvid[dst]}
			bucket := buckets[k]
			idx := cursor[k]
			if idx >= len(bucket) {
				// Not found (e.g. invisible edge swallowed by dot, or a
				// container edge we mapped to a descendant that didn't
				// produce output). Skip — edge.Route stays empty.
				continue
			}
			cursor[k] = idx + 1
			dot := bucket[idx]

			pts, err := parseSplinePos(dot.Pos)
			if err != nil {
				return fmt.Errorf("parse edge pos for %s->%s: %w", src, dst, err)
			}
			if len(pts) == 0 {
				continue
			}
			route := make([]*geo.Point, len(pts))
			for i, p := range pts {
				route[i] = geo.NewPoint(math.Round(p[0]), math.Round(flipY(p[1])))
			}
			edge.Route = route
			edge.IsCurve = len(route) >= 4 // dot splines are cubic Beziers
		}
	}

	return nil
}

// parseFloat is a small helper that swallows errors — callers treat
// missing values as zero.
func parseFloat(s string) (float64, error) {
	if s == "" {
		return 0, nil
	}
	var v float64
	_, err := fmt.Sscanf(s, "%f", &v)
	return v, err
}

// positionLabels mirrors the dagre plugin's positionLabelsIcons +
// edge-label default behaviour. Every object and edge that has a
// label must have LabelPosition set before D2's router and exporter
// run, otherwise they nil-deref.
func positionLabels(g *d2graph.Graph) {
	strPtr := func(s string) *string { return &s }

	for _, obj := range g.Objects {
		// Icon default — mirrored from dagre.
		if obj.Icon != nil && obj.IconPosition == nil {
			switch {
			case len(obj.ChildrenArray) > 0:
				obj.IconPosition = strPtr(label.OutsideTopLeft.String())
			case obj.SQLTable != nil || obj.Class != nil || obj.Language != "":
				obj.IconPosition = strPtr(label.OutsideTopLeft.String())
			default:
				obj.IconPosition = strPtr(label.InsideMiddleCenter.String())
			}
		}
		if !obj.HasLabel() || obj.LabelPosition != nil {
			continue
		}
		switch {
		case len(obj.ChildrenArray) > 0:
			obj.LabelPosition = strPtr(label.OutsideTopCenter.String())
		case obj.HasOutsideBottomLabel():
			obj.LabelPosition = strPtr(label.OutsideBottomCenter.String())
		case obj.Icon != nil:
			obj.LabelPosition = strPtr(label.InsideTopCenter.String())
		default:
			obj.LabelPosition = strPtr(label.InsideMiddleCenter.String())
		}
		// If the label is bigger than its shape, push it outside.
		if float64(obj.LabelDimensions.Width) > obj.Width || float64(obj.LabelDimensions.Height) > obj.Height {
			if len(obj.ChildrenArray) > 0 {
				obj.LabelPosition = strPtr(label.OutsideTopCenter.String())
			} else {
				obj.LabelPosition = strPtr(label.OutsideBottomCenter.String())
			}
		}
	}

	for _, e := range g.Edges {
		if e.Label.Value != "" && e.LabelPosition == nil {
			e.LabelPosition = strPtr(label.InsideMiddleCenter.String())
		}
	}
}
