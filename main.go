package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	configFilePath = flag.String("config", "config.yaml", "Path to the config file.")
)

func main() {
	cfg, err := loadConfig(*configFilePath)
	if err != nil {
		log.Fatal(err)
	}

	reload := make(chan os.Signal, 1)
	go func(sigs chan os.Signal) {
		for range sigs {
			log.Print("Config file modified, reloading")
			if err := cfg.reload(); err != nil {
				log.Fatalf("Failed to reload updated config: %v", err)
			}
		}
	}(reload)
	signal.Notify(reload, syscall.SIGHUP)

	exit := make(chan os.Signal, 1)
	signal.Notify(exit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	addr := fmt.Sprintf("%s:%d", cfg.ListenAddress, cfg.ListenPort)
	srv := &http.Server{
		Addr:    addr,
		Handler: cfg,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	log.Printf("Listening on %s", addr)

	<-exit
	log.Print("SIGTERM received, shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal(err)
	}
}

func init() {
	flag.Parse()
}
