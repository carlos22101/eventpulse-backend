package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/eventpulse/backend/config"
	"github.com/eventpulse/backend/internal/auth"
	"github.com/eventpulse/backend/internal/db"
	"github.com/eventpulse/backend/internal/handlers"
	"github.com/eventpulse/backend/internal/middleware"
	"github.com/eventpulse/backend/internal/repository"
	"github.com/eventpulse/backend/internal/ws"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	// ── Cargar configuración ──────────────────────────────────────────────────
	cfg := config.Load()

	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// ── Conexiones ────────────────────────────────────────────────────────────
	postgres := db.NewPostgres(cfg)
	redisClient := db.NewRedis(cfg)
	defer postgres.Close()

	// ── Repositorios ──────────────────────────────────────────────────────────
	usuarioRepo := repository.NewUsuarioRepository(postgres)
	incidenciaRepo := repository.NewIncidenciaRepository(postgres)
	tareaRepo := repository.NewTareaRepository(postgres)
	mensajeRepo := repository.NewMensajeRepository(postgres)

	// ── Servicios ─────────────────────────────────────────────────────────────
	jwtSvc := auth.NewJWTService(cfg)

	// ── WebSocket Hub ─────────────────────────────────────────────────────────
	hub := ws.NewHub(redisClient, cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	// ── Handlers ──────────────────────────────────────────────────────────────
	authHandler := handlers.NewAuthHandler(usuarioRepo, jwtSvc)
	incidenciaHandler := handlers.NewIncidenciaHandler(incidenciaRepo, hub)
	tareaHandler := handlers.NewTareaHandler(tareaRepo, hub)
	chatHandler := handlers.NewChatHandler(mensajeRepo, usuarioRepo, hub)
	wsHandler := handlers.NewWSHandler(hub, jwtSvc)

	// ── Router ────────────────────────────────────────────────────────────────
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// CORS - permitir Android y web
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"version": "1.0.0",
		})
	})

	// ── Rutas públicas ────────────────────────────────────────────────────────
	api := r.Group("/api/v1")
	{
		api.POST("/auth/login", authHandler.Login)
	}

	// ── WebSocket (auth por query param) ─────────────────────────────────────
	r.GET("/ws", wsHandler.Conectar)

	// ── Rutas protegidas ──────────────────────────────────────────────────────
	protected := api.Group("")
	protected.Use(middleware.Auth(jwtSvc))
	{
		// Perfil
		protected.GET("/auth/me", authHandler.Me)

		// Incidencias
		incidencias := protected.Group("/incidencias")
		{
			incidencias.GET("", incidenciaHandler.Listar)
			incidencias.POST("", incidenciaHandler.Reportar)
			incidencias.PATCH("/:id/atender", incidenciaHandler.Atender)
			incidencias.PATCH("/:id/resolver", incidenciaHandler.Resolver)
		}

		// Tareas
		tareas := protected.Group("/tareas")
		{
			tareas.GET("", tareaHandler.Listar)
			tareas.PATCH("/:id/completar", tareaHandler.Completar)
		}

		// Chat
		chat := protected.Group("/chat")
		{
			chat.GET("/historial", chatHandler.Historial)
			chat.POST("/mensaje", chatHandler.Enviar)
		}
	}

	// ── Servidor con graceful shutdown ────────────────────────────────────────
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Arrancar servidor en goroutine
	go func() {
		fmt.Printf("🚀 EventPulse backend corriendo en puerto %s\n", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error servidor: %v", err)
		}
	}()

	// Esperar señal de shutdown (SIGINT o SIGTERM de Docker/AWS)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("⏳ Apagando servidor...")
	cancel() // Detener el hub de WebSocket

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Error en graceful shutdown: %v", err)
	}

	fmt.Println("✅ Servidor apagado correctamente")
}
