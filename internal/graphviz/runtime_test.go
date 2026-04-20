package graphviz

import (
	"reflect"
	"testing"
)

func TestParsePoint(t *testing.T) {
	x, y, err := parsePoint("12.5,30.0")
	if err != nil {
		t.Fatal(err)
	}
	if x != 12.5 || y != 30.0 {
		t.Fatalf("got (%v,%v)", x, y)
	}
	if _, _, err := parsePoint("bad"); err == nil {
		t.Fatal("expected error for malformed point")
	}
}

func TestParseBB(t *testing.T) {
	x1, y1, x2, y2, err := parseBB("0,0,100,200")
	if err != nil {
		t.Fatal(err)
	}
	if x1 != 0 || y1 != 0 || x2 != 100 || y2 != 200 {
		t.Fatalf("got (%v,%v,%v,%v)", x1, y1, x2, y2)
	}
}

func TestParseSplinePos_PolylineOnly(t *testing.T) {
	// Three control points, no explicit arrow start/end overrides.
	got, err := parseSplinePos("10,20 30,40 50,60")
	if err != nil {
		t.Fatal(err)
	}
	want := [][2]float64{{10, 20}, {30, 40}, {50, 60}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestParseSplinePos_WithEndpointOverrides(t *testing.T) {
	// "e,ex,ey" tells Graphviz to attach the arrow at (ex,ey) instead
	// of the last control point; likewise for "s,sx,sy" at the start.
	got, err := parseSplinePos("e,100,100 s,0,0 10,20 30,40 50,60 70,80")
	if err != nil {
		t.Fatal(err)
	}
	// Expect first control replaced with s, last replaced with e.
	want := [][2]float64{{0, 0}, {30, 40}, {50, 60}, {100, 100}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestParseSplinePos_Empty(t *testing.T) {
	got, err := parseSplinePos("")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("expected nil for empty pos, got %v", got)
	}
}
