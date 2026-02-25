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

func NewPostgres(cfg *config.Config) *sqlx.DB {
	db, err := sqlx.Connect("postgres", cfg.DB.DSN())
	if err != nil {
		log.Fatalf("Error conectando PostgreSQL: %v", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	fmt.Println("✅ PostgreSQL conectado")
	return db
}

func NewRedis(cfg *config.Config) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := client.Ping(ctx).Result(); err != nil {
		log.Fatalf("Error conectando Redis: %v", err)
	}
	fmt.Println("✅ Redis conectado")
	return client
}
