package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"github.com/iwittkau/proto-golint/pkg/analyzer"
)

func main() {
	singlechecker.Main(analyzer.ProtoGetters)
}
