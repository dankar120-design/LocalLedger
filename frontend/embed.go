package frontend

import "embed"

//go:embed static/* views/*
var FS embed.FS
