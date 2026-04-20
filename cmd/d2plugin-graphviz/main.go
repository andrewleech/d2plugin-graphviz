package main

import (
	"oss.terrastruct.com/util-go/xmain"

	"github.com/andrewleech/d2plugin-graphviz/internal/graphviz"
)

var Version = "dev"

func main() {
	xmain.Main(graphviz.Serve(Version))
}
