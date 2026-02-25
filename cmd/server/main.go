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
	cfg := config.Load()
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// ── Conexiones ────────────────────────────────────────────────────────────
	postgres := db.NewPostgres(cfg)
	redisClient := db.NewRedis(cfg)
	defer postgres.Close()

	// ── Repositorios ──────────────────────────────────────────────────────────
	usuarioRepo := repository.NewUsuarioRepo(postgres)
	eventoRepo := repository.NewEventoRepo(postgres)
	zonaRepo := repository.NewZonaRepo(postgres)
	incidenciaRepo := repository.NewIncidenciaRepo(postgres)
	tareaRepo := repository.NewTareaRepo(postgres)
	mensajeRepo := repository.NewMensajeRepo(postgres)

	// ── Servicios ─────────────────────────────────────────────────────────────
	jwtSvc := auth.NewJWTService(cfg)

	// ── WebSocket Hub ─────────────────────────────────────────────────────────
	hub := ws.NewHub(redisClient, cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	// ── Handlers ──────────────────────────────────────────────────────────────
	authH := handlers.NewAuthHandler(usuarioRepo, eventoRepo, jwtSvc)
	eventoH := handlers.NewEventoHandler(eventoRepo, usuarioRepo, hub)
	usuarioH := handlers.NewUsuarioHandler(usuarioRepo, eventoRepo)
	zonaH := handlers.NewZonaHandler(zonaRepo, eventoRepo)
	incidenciaH := handlers.NewIncidenciaHandler(incidenciaRepo, eventoRepo, hub)
	tareaH := handlers.NewTareaHandler(tareaRepo, eventoRepo, hub)
	chatH := handlers.NewChatHandler(mensajeRepo, eventoRepo, hub)
	wsH := handlers.NewWSHandler(hub, jwtSvc, eventoRepo)

	// ── Router ────────────────────────────────────────────────────────────────
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	r.Use(cors.New(cors.Config{
		AllowOrigins:  []string{"*"},
		AllowMethods:  []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:  []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders: []string{"Content-Length"},
		MaxAge:        12 * time.Hour,
	}))

	// Health check público
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "version": "2.0.0"})
	})

	// WebSocket — auth por query param ?token=
	r.GET("/ws", wsH.Conectar)

	// ── API v1 ────────────────────────────────────────────────────────────────
	api := r.Group("/api/v1")

	// ── Rutas públicas ────────────────────────────────────────────────────────
	api.POST("/auth/login", authH.Login)

	// ── Rutas protegidas (cualquier usuario autenticado) ──────────────────────
	auth := api.Group("")
	auth.Use(middleware.Auth(jwtSvc))
	{
		// Perfil propio
		auth.GET("/auth/me", authH.Me)

		// Eventos — admin ve todos, trabajador ve solo el suyo
		auth.GET("/eventos", eventoH.Listar)

		// Zonas
		auth.GET("/zonas", zonaH.Listar)

		// Incidencias — todos pueden ver y editar estado
		auth.GET("/incidencias", incidenciaH.Listar)
		auth.GET("/incidencias/:id", incidenciaH.ObtenerPorID)
		auth.PATCH("/incidencias/:id", incidenciaH.Editar)

		// Tareas — todos pueden ver y editar estado
		auth.GET("/tareas", tareaH.Listar)
		auth.GET("/tareas/:id", tareaH.ObtenerPorID)
		auth.PATCH("/tareas/:id", tareaH.Editar)

		// Chat — admin y trabajadores
		auth.GET("/chat/historial", chatH.Historial)
		auth.POST("/chat/mensaje", chatH.Enviar)
	}

	// ── Rutas solo admin ──────────────────────────────────────────────────────
	admin := api.Group("")
	admin.Use(middleware.Auth(jwtSvc), middleware.SoloAdmin())
	{
		// Gestión de eventos
		admin.POST("/eventos", eventoH.Crear)
		admin.PATCH("/eventos/:id/terminar", eventoH.Terminar)

		// Gestión de usuarios (crear staff)
		admin.POST("/usuarios", usuarioH.Crear)
		admin.GET("/usuarios", usuarioH.Listar)

		// Gestión de zonas
		admin.POST("/zonas", zonaH.Crear)
		admin.DELETE("/zonas/:id", zonaH.Eliminar)

		// Crear incidencias y tareas (solo admin)
		admin.POST("/incidencias", incidenciaH.Crear)
		admin.POST("/tareas", tareaH.Crear)
	}

	// ── Servidor con graceful shutdown ────────────────────────────────────────
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		fmt.Printf("🚀 EventPulse v2 corriendo en puerto %s\n", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error servidor: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("⏳ Apagando servidor...")
	cancel()
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Fatalf("Shutdown error: %v", err)
	}
	fmt.Println("✅ Servidor apagado")
}
