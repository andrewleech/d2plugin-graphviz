package config

import (
	"reflect"
	"testing"
)

func TestParseDataConfig_JSONString(t *testing.T) {
	got := ParseDataConfig(map[string]interface{}{
		"graphviz": `{
			"global_attrs": {"rankdir": "LR", "nodesep": "0.2"},
			"rank_groups": [["a", "b"], ["c", "d"]],
			"node_attrs": {"x": {"weight": "10"}},
			"edge_attrs": {"a -> b": {"constraint": "false"}}
		}`,
	})
	want := DataConfig{
		GlobalAttrs: map[string]string{"rankdir": "LR", "nodesep": "0.2"},
		RankGroups:  [][]string{{"a", "b"}, {"c", "d"}},
		NodeAttrs:   map[string]map[string]string{"x": {"weight": "10"}},
		EdgeAttrs:   map[string]map[string]string{"a -> b": {"constraint": "false"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v\nwant %#v", got, want)
	}
}

func TestParseDataConfig_EmptyOrMissing(t *testing.T) {
	cases := []map[string]interface{}{
		nil,
		{},
		{"graphviz": ""},
		{"graphviz": "not-json"},
		{"other": "ignored"},
	}
	for _, c := range cases {
		got := ParseDataConfig(c)
		if !reflect.DeepEqual(got, DataConfig{}) {
			t.Errorf("expected empty DataConfig for %v, got %#v", c, got)
		}
	}
}

func TestBuiltinClasses(t *testing.T) {
	if BuiltinClasses["no-constraint"]["constraint"] != "false" {
		t.Error("no-constraint should set constraint=false")
	}
	if BuiltinClasses["bold-path"]["weight"] == "" {
		t.Error("bold-path should have weight attr")
	}
}

func TestCLIOpts_Attr(t *testing.T) {
	o := CLIOpts{
		"graphviz-rankdir": "LR",
		"graphviz-nodesep": "",
	}
	if v, ok := o.Attr("rankdir"); !ok || v != "LR" {
		t.Errorf("rankdir: got (%q, %v)", v, ok)
	}
	if _, ok := o.Attr("nodesep"); ok {
		t.Error("empty values should count as unset")
	}
	if _, ok := o.Attr("unknown"); ok {
		t.Error("missing key should count as unset")
	}
}
