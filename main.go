package main

import (
	"embed"

	"github.com/s0x401/xdfile-manager/src/cmd"
)

var (
	//go:embed src/xdfile_config/*
	content embed.FS
)

func main() {
	cmd.Run(content)
}
