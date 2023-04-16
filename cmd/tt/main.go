package main

import (
	"log"
	"os"
	"net/http"

	"github.com/hugomfc/tt/internal/config"
	ttHttpHandler "github.com/hugomfc/tt/internal/interfaces/http"
	"github.com/hugomfc/tt/internal/ratelimiter"
	"github.com/hugomfc/tt/internal/storage"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	redisStorage, err := storage.NewRedis(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB, cfg.Redis.CounterTTL)
	if err != nil {
    log.Fatalf("failed to connect to redis: %v", cfg.Redis)
  }

  rl := ratelimiter.New(redisStorage, 5, 60)

	//if err := rl.GetRules(context.Background()); err != nil {
	//	log.Fatalf("failed to load rules: %v", err)
	//}

	httpServer := ttHttpHandler.NewHttpHandler(rl)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Listening on :%s...", port)
	log.Fatal(http.ListenAndServe(":"+port, httpServer))
}

