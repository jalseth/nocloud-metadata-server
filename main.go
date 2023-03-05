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

	"github.com/fsnotify/fsnotify"
)

var (
	configFilePath = flag.String("config", "config.yaml", "Path to the config file.")
)

func main() {
	cfg, err := loadConfig(*configFilePath)
	if err != nil {
		log.Fatal(err)
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	go func() {
		for {
			select {
			case event, more := <-watcher.Events:
				if !more {
					return
				}
				if event.Has(fsnotify.Write) {
					log.Print("Config file modified, reloading")
					if err := cfg.reload(); err != nil {
						log.Fatalf("Failed to reload updated config: %v", err)
					}
				}
			case err, more := <-watcher.Errors:
				if !more {
					return
				}
				log.Printf("WARN: Config watcher encountered an error: %v", err)
			}
		}
	}()
	if err := watcher.Add(*configFilePath); err != nil {
		log.Fatal(err)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

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

	<-sig
	close(watcher.Events)
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
