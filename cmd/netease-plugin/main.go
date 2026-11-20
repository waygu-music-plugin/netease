package main

import (
	"log"

	"github.com/waygu-music-plugin/sdk/desktop"
	"github.com/waygu-music-plugin/netease"
)

func main() {
	if err := desktop.RunPlatformPlugin(netease.Registration()); err != nil {
		log.Fatal(err)
	}
}
