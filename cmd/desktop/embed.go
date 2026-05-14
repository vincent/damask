//go:build desktop && !dev

package main

import "embed"

//go:embed all:frontend/dist
var assets embed.FS
