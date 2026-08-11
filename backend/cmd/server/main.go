package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/browser-gateway/backend/internal/config"
	"github.com/browser-gateway/backend/internal/httpserver"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	app, err := httpserver.New(cfg)
	if err != nil {
		log.Fatalf("server: %v", err)
	}

	go func() {
		if err := app.Listen(cfg.HTTPAddr); err != nil && err.Error() != "http: Server closed" {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("shutting down")
	_ = app.Shutdown()
}
