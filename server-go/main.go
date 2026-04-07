package main

import (
	"log"

	"github.com/homepy/hwarr/server-go/internal/config"
	"github.com/homepy/hwarr/server-go/internal/server"
)

func main() {
	cfg := config.Load()
	if err := server.Run(cfg); err != nil {
		log.Fatal(err)
	}
}
