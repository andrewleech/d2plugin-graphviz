package graphviz

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// dotOutput is the subset of `dot -Tjson` fields we consume. Graphviz's
// xdot JSON schema is well-documented at
// https://graphviz.org/docs/outputs/json/. We rely on these keys:
//   bb           — overall graph bounding box "x1,y1,x2,y2" in points
//   objects[]    — nodes and subgraphs, interleaved
//   edges[]      — edges
//
// Each object has _gvid, name (our AbsID), pos, width, height. Subgraphs
// additionally have bb and a subgraphs/nodes member linking their
// _gvids.
type dotOutput struct {
	BB      string        `json:"bb"`
	Objects []dotObject   `json:"objects"`
	Edges   []dotEdge     `json:"edges"`
}

type dotObject struct {
	Gvid   int    `json:"_gvid"`
	Name   string `json:"name"`
	Pos    string `json:"pos"`    // "x,y" centre, in points
	Width  string `json:"width"`  // inches
	Height string `json:"height"` // inches
	BB     string `json:"bb"`     // subgraphs only, "x1,y1,x2,y2" points
	// Presence of any of these implies the object is a subgraph.
	Subgraphs []int `json:"subgraphs,omitempty"`
	Nodes     []int `json:"nodes,omitempty"`
	Edges     []int `json:"edges,omitempty"`
}

type dotEdge struct {
	Gvid int    `json:"_gvid"`
	Tail int    `json:"tail"`
	Head int    `json:"head"`
	Pos  string `json:"pos"` // B-spline: "e,ex,ey p1,p1y p2,p2y ..." optionally s,... at front
}

// runDot feeds dotSrc to `dot -Tjson` and returns the parsed output.
// Any stderr output from dot is appended to the error for diagnosis.
func runDot(ctx context.Context, dotSrc string) (*dotOutput, error) {
	cmd := exec.CommandContext(ctx, "dot", "-Tjson")
	cmd.Stdin = strings.NewReader(dotSrc)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// Surface stderr to the user — dot's messages are usually the
		// most useful debugging signal.
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("dot failed: %v\n%s", err, stderr.String())
		}
		return nil, fmt.Errorf("dot failed: %v", err)
	}
	var out dotOutput
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		return nil, fmt.Errorf("parse dot json: %w", err)
	}
	return &out, nil
}

// parsePoint splits an "x,y" pair into floats.
func parsePoint(s string) (float64, float64, error) {
	parts := strings.SplitN(s, ",", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("bad point %q", s)
	}
	x, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return 0, 0, err
	}
	y, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return 0, 0, err
	}
	return x, y, nil
}

// parseBB splits "x1,y1,x2,y2" into min/max.
func parseBB(s string) (x1, y1, x2, y2 float64, err error) {
	parts := strings.Split(s, ",")
	if len(parts) != 4 {
		err = fmt.Errorf("bad bb %q", s)
		return
	}
	vals := make([]float64, 4)
	for i, p := range parts {
		v, e := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if e != nil {
			err = e
			return
		}
		vals[i] = v
	}
	return vals[0], vals[1], vals[2], vals[3], nil
}

// parseSplinePos parses an edge `pos` attribute. Format:
//
//	[s,sx,sy ][e,ex,ey ]p0 p1 p2 ... pn
//
// where p0..pn are B-spline control points ("x,y") with n == 3k for
// some k ≥ 1 (each segment uses 4 points, sharing endpoints). The
// optional "s,..." and "e,..." entries override the logical start/end
// with arrow-head attachment points. We use them when present because
// they land exactly on the shape border, which is what D2's renderer
// expects.
func parseSplinePos(pos string) ([][2]float64, error) {
	if pos == "" {
		return nil, nil
	}
	tokens := strings.Fields(pos)
	var start, end *[2]float64
	var ctrl [][2]float64
	for _, t := range tokens {
		if strings.HasPrefix(t, "s,") {
			x, y, err := parsePoint(strings.TrimPrefix(t, "s,"))
			if err != nil {
				return nil, err
			}
			start = &[2]float64{x, y}
			continue
		}
		if strings.HasPrefix(t, "e,") {
			x, y, err := parsePoint(strings.TrimPrefix(t, "e,"))
			if err != nil {
				return nil, err
			}
			end = &[2]float64{x, y}
			continue
		}
		x, y, err := parsePoint(t)
		if err != nil {
			return nil, err
		}
		ctrl = append(ctrl, [2]float64{x, y})
	}
	// Replace the first and last control points with the explicit
	// arrow start/end if present.
	if start != nil && len(ctrl) > 0 {
		ctrl[0] = *start
	}
	if end != nil && len(ctrl) > 0 {
		ctrl[len(ctrl)-1] = *end
	}
	return ctrl, nil
}
