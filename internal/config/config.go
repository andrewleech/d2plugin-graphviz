// Package config holds the plugin's CLI flag definitions and the parsed
// graph-wide graphviz config channel (read later from g.Data["graphviz"]).
package config

import (
	"strings"

	"oss.terrastruct.com/d2/d2plugin"
)

const cliPrefix = "graphviz-"

// CLIOpts holds the user-supplied CLI flag values after hydration. Keys
// are the flag names (e.g. "graphviz-rankdir"); the bare Graphviz attr
// names (e.g. "rankdir") can be recovered with Attr().
//
// Values arrive as strings — the D2 CLI stringifies even int64 flags
// when transferring them to external plugin binaries (see
// d2plugin/exec.go HydrateOpts + Layout). Keeping them as strings
// preserves the full Graphviz attribute grammar (e.g. `splines=ortho`,
// `ratio=compress`).
type CLIOpts map[string]string

// Attr returns the Graphviz attribute value for the given bare attr
// name (without the "graphviz-" prefix), plus whether it was set by
// the user. Empty strings count as unset so that Graphviz's own
// defaults apply.
func (o CLIOpts) Attr(name string) (string, bool) {
	v, ok := o[cliPrefix+name]
	if !ok || v == "" {
		return "", false
	}
	return v, true
}

// AttrsNonEmpty returns the subset of user-supplied CLI flags that have
// non-empty values, keyed by the bare Graphviz attr name.
func (o CLIOpts) AttrsNonEmpty() map[string]string {
	out := map[string]string{}
	for k, v := range o {
		if v == "" {
			continue
		}
		out[strings.TrimPrefix(k, cliPrefix)] = v
	}
	return out
}

// DefaultCLIOpts returns a zero-valued set. Defaults live in CLIFlags so
// that the d2 CLI sees them in --help output.
func DefaultCLIOpts() CLIOpts { return CLIOpts{} }

// flag is a compact internal description of a single CLI flag, expanded
// into d2plugin.PluginSpecificFlag by CLIFlags. Name == Tag is required
// for external plugins (see note in CLIFlags).
type flag struct {
	attr    string      // bare Graphviz attribute (e.g. "rankdir")
	typ     string      // "string" or "int64"
	dflt    interface{} // default value
	usage   string
}

func (f flag) toPlugin() d2plugin.PluginSpecificFlag {
	name := cliPrefix + f.attr
	return d2plugin.PluginSpecificFlag{
		Name:    name,
		Tag:     name, // see note below
		Type:    f.typ,
		Default: f.dflt,
		Usage:   f.usage,
	}
}

// CLIFlags declares the plugin's CLI flags.
//
// A subtle but critical constraint: for an **external** plugin (binary
// on $PATH discovered via `d2plugin-<name>`), the D2 CLI transfers user
// flag values to the plugin subprocess using `--<Tag> <value>` argv
// entries (d2plugin/exec.go execPlugin.Layout). The plugin's own
// argv parser, however, is registered from `Name` via
// PluginSpecificFlag.AddToOpts. If Name ≠ Tag, the plugin's argv parse
// fails with "unknown flag".
//
// We therefore set Name == Tag == "graphviz-<attr>" — prefixed for
// uniqueness in D2's global flag namespace, and aligned so argv parsing
// works both sides.
func CLIFlags() []d2plugin.PluginSpecificFlag {
	defs := []flag{
		{"rankdir", "string", "",
			"Graphviz rankdir (TB, BT, LR, RL). Empty derives from D2 root direction."},
		{"nodesep", "string", "",
			"Minimum space between sibling nodes (inches). Empty uses Graphviz default (0.25)."},
		{"ranksep", "string", "",
			"Minimum space between ranks (inches). Empty uses Graphviz default (0.5)."},
		{"splines", "string", "true",
			"Edge routing: true, false, polyline, ortho, spline, curved."},
		{"size", "string", "",
			"Canvas size hint in inches (e.g. '7.3,10.2'). Interacts with ratio."},
		{"ratio", "string", "",
			"Aspect ratio: fill, compress, expand, auto, or a number."},
		{"dpi", "string", "72",
			"DPI used for the dot→pixel conversion. 72 matches D2's native pixel space."},
		{"newrank", "string", "true",
			"Enable Graphviz newrank. Improves rank=same across clusters."},
		{"concentrate", "string", "false",
			"Merge parallel edges."},
		{"overlap", "string", "",
			"Overlap resolution strategy (mostly for non-dot engines)."},
		{"margin", "string", "",
			"Graph-wide margin."},
		{"pad", "string", "",
			"Graph-wide pad."},
	}
	out := make([]d2plugin.PluginSpecificFlag, len(defs))
	for i, f := range defs {
		out[i] = f.toPlugin()
	}
	return out
}
