package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
)

var (
	configFilePath = flag.String("config", "config.yaml", "Path to the config file.")
)

func main() {
	cfg, err := loadConfig(*configFilePath)
	if err != nil {
		log.Fatal(err)
	}

	addr := fmt.Sprintf("%s:%d", cfg.ListenAddress, cfg.ListenPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Listening on %s", addr)
	if err := http.Serve(listener, cfg); err != nil {
		log.Fatal(err)
	}
}

func init() {
	flag.Parse()
}
