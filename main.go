package main

import (
	"github.com/awilcots/envreport/cmd/envreport"

	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(envreport.Analyzer)
}
