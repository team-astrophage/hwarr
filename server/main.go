package main

import (
	"log"

	"github.com/homepy/hwarr/server/internal/config"
	"github.com/homepy/hwarr/server/internal/server"
)

func main() {
	cfg := config.Load()
	if err := server.Run(cfg); err != nil {
		log.Fatal(err)
	}
}
