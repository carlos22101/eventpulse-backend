package db

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/eventpulse/backend/config"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

// ─── PostgreSQL ───────────────────────────────────────────────────────────────

func NewPostgres(cfg *config.Config) *sqlx.DB {
	db, err := sqlx.Connect("postgres", cfg.DB.DSN())
	if err != nil {
		log.Fatalf("Error conectando a PostgreSQL: %v", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		log.Fatalf("No se puede hacer ping a PostgreSQL: %v", err)
	}

	fmt.Println("✅ PostgreSQL conectado")
	return db
}

// ─── Redis ────────────────────────────────────────────────────────────────────

func NewRedis(cfg *config.Config) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := client.Ping(ctx).Result(); err != nil {
		log.Fatalf("Error conectando a Redis: %v", err)
	}

	fmt.Println("✅ Redis conectado")
	return client
}
