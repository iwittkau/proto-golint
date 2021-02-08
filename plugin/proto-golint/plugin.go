// This must be package main
package main

import (
	"golang.org/x/tools/go/analysis"

	"github.com/iwittkau/proto-golint/pkg/analyzer"
)

type analyzerPlugin struct{}

// This must be implemented
func (*analyzerPlugin) GetAnalyzers() []*analysis.Analyzer {
	return []*analysis.Analyzer{
		analyzer.ProtoGetters,
	}
}

// AnalyzerPlugin must be defined and named 'AnalyzerPlugin'
var AnalyzerPlugin analyzerPlugin // nolint: deadcode, unused
