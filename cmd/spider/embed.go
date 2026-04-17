package main

import "embed"

//go:embed all:dist
var webFS embed.FS
