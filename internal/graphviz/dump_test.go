package graphviz

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"oss.terrastruct.com/d2/d2compiler"
	"oss.terrastruct.com/d2/d2graph"
	"oss.terrastruct.com/d2/lib/textmeasure"

	"github.com/andrewleech/d2plugin-graphviz/internal/config"
)

// TestDumpDot is a developer aid: `go test -run TestDumpDot -v -dot=examples/input.d2`
// prints the .dot source that BuildDot emits for the given D2 file.
// Skipped when the -dot flag isn't set so `go test` stays clean.
func TestDumpDot(t *testing.T) {
	path := os.Getenv("DUMP_DOT_INPUT")
	if path == "" {
		t.Skip("set DUMP_DOT_INPUT to a D2 file path to dump its emitted dot source")
	}
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	ruler, err := textmeasure.NewRuler()
	if err != nil {
		t.Fatal(err)
	}
	// Use d2compiler directly (not d2lib) so we skip running a layout —
	// we only want the parsed graph + measured labels. d2lib insists on
	// running a registered layout engine, which creates a circular
	// dependency for testing this plugin in isolation.
	//
	// Feed g.Data from the compiler config so tests can exercise the
	// vars.d2-config.data.graphviz.* channel without going through the
	// full d2 CLI.
	g, compilerConfig, err := d2compiler.Compile(path, strings.NewReader(string(src)), nil)
	if err != nil {
		t.Fatal(err)
	}
	if compilerConfig != nil {
		g.Data = compilerConfig.Data
	}
	if err := g.SetDimensions(nil, ruler, nil, nil); err != nil {
		t.Fatal(err)
	}
	// Round-trip through serde so the Object tree is rebuilt the same
	// way it would be when the plugin receives stdin.
	b, err := d2graph.SerializeGraph(g)
	if err != nil {
		t.Fatal(err)
	}
	var g2 d2graph.Graph
	if err := d2graph.DeserializeGraph(b, &g2); err != nil {
		t.Fatal(err)
	}
	dot, err := BuildDot(&g2, config.DefaultCLIOpts())
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(dot)
}
