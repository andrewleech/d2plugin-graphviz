// Package graphviz implements a D2 layout plugin backed by the Graphviz
// `dot` engine. It wires the d2plugin.Plugin interface and delegates the
// actual translation to the sibling files in this package.
package graphviz

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"oss.terrastruct.com/d2/d2graph"
	"oss.terrastruct.com/d2/d2plugin"
	"oss.terrastruct.com/util-go/xmain"

	"github.com/andrewleech/d2plugin-graphviz/internal/config"
)

const pluginName = "graphviz"

const longHelp = `Layout engine using Graphviz's dot.

This plugin shells out to the system ` + "`dot`" + ` binary, converts the D2
graph to a .dot source, runs layout, and maps positions back. D2 renders
the result with its own SVG renderer.

Requires the ` + "`dot`" + ` binary to be available in $PATH (install the
Graphviz package for your platform).

Graph-wide attrs, rank hints, and per-element overrides are read from
` + "`vars.d2-config.data.graphviz.*`" + ` in the D2 source. See the plugin
README for the full reference.`

type plugin struct {
	version string

	mu   sync.Mutex
	opts config.CLIOpts
}

// Serve builds the xmain.RunFunc used by cmd/d2plugin-graphviz/main.go.
func Serve(version string) xmain.RunFunc {
	return d2plugin.Serve(&plugin{version: version})
}

func (p *plugin) Info(ctx context.Context) (*d2plugin.PluginInfo, error) {
	opts := xmain.NewOpts(nil, nil)
	flags, err := p.Flags(ctx)
	if err != nil {
		return nil, err
	}
	for _, f := range flags {
		f.AddToOpts(opts)
	}
	return &d2plugin.PluginInfo{
		Name:      pluginName,
		ShortHelp: "Graphviz dot layout engine for D2",
		LongHelp:  fmt.Sprintf("%s\n\nFlags:\n%s", longHelp, opts.Defaults()),
		Features:  []d2plugin.PluginFeature{},
	}, nil
}

func (p *plugin) Flags(context.Context) ([]d2plugin.PluginSpecificFlag, error) {
	return config.CLIFlags(), nil
}

func (p *plugin) HydrateOpts(raw []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if raw == nil {
		p.opts = config.DefaultCLIOpts()
		return nil
	}
	o := config.CLIOpts{}
	if err := json.Unmarshal(raw, &o); err != nil {
		return xmain.UsageErrorf("invalid graphviz plugin options: %v", err)
	}
	p.opts = o
	return nil
}

func (p *plugin) Layout(ctx context.Context, g *d2graph.Graph) error {
	p.mu.Lock()
	opts := p.opts
	p.mu.Unlock()
	return Layout(ctx, g, opts)
}

func (p *plugin) PostProcess(_ context.Context, in []byte) ([]byte, error) {
	return in, nil
}
