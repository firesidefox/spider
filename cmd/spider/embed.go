package main

import "embed"

//go:embed all:dist
var webFS embed.FS

//go:embed all:skills
var builtinSkillsFS embed.FS
