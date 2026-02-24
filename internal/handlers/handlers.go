package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/eventpulse/backend/internal/auth"
	"github.com/eventpulse/backend/internal/middleware"
	"github.com/eventpulse/backend/internal/models"
	"github.com/eventpulse/backend/internal/repository"
	"github.com/eventpulse/backend/internal/ws"
	"github.com/gin-gonic/gin"
)

// ─── Auth Handler ─────────────────────────────────────────────────────────────

type AuthHandler struct {
	usuarioRepo *repository.UsuarioRepository
	jwtSvc     *auth.JWTService
}

func NewAuthHandler(r *repository.UsuarioRepository, j *auth.JWTService) *AuthHandler {
	return &AuthHandler{usuarioRepo: r, jwtSvc: j}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	usuario, err := h.usuarioRepo.BuscarPorEmail(c.Request.Context(), req.Email)
	if err != nil || usuario == nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Credenciales inválidas"})
		return
	}

	if !h.usuarioRepo.ValidarPassword(usuario.Password, req.Password) {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Credenciales inválidas"})
		return
	}

	token, err := h.jwtSvc.GenerarToken(usuario)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Error generando token"})
		return
	}

	usuario.Password = "" // No enviar el hash
	c.JSON(http.StatusOK, models.LoginResponse{Token: token, Usuario: *usuario})
}

func (h *AuthHandler) Me(c *gin.Context) {
	usuarioID := middleware.GetUsuarioID(c)
	usuario, err := h.usuarioRepo.BuscarPorID(c.Request.Context(), usuarioID)
	if err != nil || usuario == nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Usuario no encontrado"})
		return
	}
	usuario.Password = ""
	c.JSON(http.StatusOK, usuario)
}

// ─── Incidencia Handler ───────────────────────────────────────────────────────

type IncidenciaHandler struct {
	repo *repository.IncidenciaRepository
	hub  *ws.Hub
}

func NewIncidenciaHandler(r *repository.IncidenciaRepository, h *ws.Hub) *IncidenciaHandler {
	return &IncidenciaHandler{repo: r, hub: h}
}

func (h *IncidenciaHandler) Listar(c *gin.Context) {
	eventoID := middleware.GetEventoID(c)
	items, err := h.repo.Listar(c.Request.Context(), eventoID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Error listando incidencias"})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *IncidenciaHandler) Reportar(c *gin.Context) {
	var req models.ReportarIncidenciaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	usuarioID := middleware.GetUsuarioID(c)
	eventoID := middleware.GetEventoID(c)

	nueva, err := h.repo.Crear(c.Request.Context(), &models.Incidencia{
		EventoID:    eventoID,
		ZonaID:      req.ZonaID,
		Tipo:        req.Tipo,
		Descripcion: req.Descripcion,
		ReportadaPor: usuarioID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Error creando incidencia"})
		return
	}

	// Publicar a todos los clientes del evento via Redis → WebSocket
	go h.hub.PublicarEvento(c.Request.Context(), eventoID, models.EventoWS{
		Tipo:     models.WSIncidenciaNueva,
		Payload:  nueva,
		EventoID: eventoID,
	})

	c.JSON(http.StatusCreated, nueva)
}

// Atender implementa el flujo optimista con manejo de conflictos.
// El cliente ya actualizó su UI; si falla, recibe un evento de rollback por WS.
func (h *IncidenciaHandler) Atender(c *gin.Context) {
	incID := c.Param("id")
	usuarioID := middleware.GetUsuarioID(c)
	eventoID := middleware.GetEventoID(c)
	ctx := c.Request.Context()

	actualizada, err := h.repo.AtenderConLock(ctx, incID, usuarioID)
	if err != nil {
		// Detectar conflicto de concurrencia
		if strings.HasPrefix(err.Error(), "incidencia_conflicto:") {
			partes := strings.SplitN(err.Error(), ":", 2)
			quien := "otro usuario"
			if len(partes) == 2 {
				quien = partes[1]
			}

			// Enviar rollback SOLO al usuario que falló
			go h.hub.PublicarAUsuario(ctx, eventoID, usuarioID, models.EventoWS{
				Tipo: models.WSIncidenciaConflicto,
				Payload: models.ConflictoPayload{
					IncidenciaID: incID,
					Mensaje:      fmt.Sprintf("La incidencia ya fue tomada por %s", quien),
					AtendidaPor:  quien,
				},
				EventoID: eventoID,
			})

			c.JSON(http.StatusConflict, models.ErrorResponse{
				Error:  fmt.Sprintf("Conflicto: ya fue tomada por %s", quien),
				Codigo: http.StatusConflict,
			})
			return
		}

		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: err.Error()})
		return
	}

	// Éxito: notificar a TODOS que la incidencia cambió de estado
	go h.hub.PublicarEvento(ctx, eventoID, models.EventoWS{
		Tipo:     models.WSIncidenciaActualizada,
		Payload:  actualizada,
		EventoID: eventoID,
	})

	c.JSON(http.StatusOK, actualizada)
}

func (h *IncidenciaHandler) Resolver(c *gin.Context) {
	incID := c.Param("id")
	usuarioID := middleware.GetUsuarioID(c)
	eventoID := middleware.GetEventoID(c)
	ctx := c.Request.Context()

	actualizada, err := h.repo.Resolver(ctx, incID, usuarioID)
	if err != nil {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: err.Error()})
		return
	}

	go h.hub.PublicarEvento(ctx, eventoID, models.EventoWS{
		Tipo:     models.WSIncidenciaActualizada,
		Payload:  actualizada,
		EventoID: eventoID,
	})

	c.JSON(http.StatusOK, actualizada)
}

// ─── Tarea Handler ────────────────────────────────────────────────────────────

type TareaHandler struct {
	repo *repository.TareaRepository
	hub  *ws.Hub
}

func NewTareaHandler(r *repository.TareaRepository, h *ws.Hub) *TareaHandler {
	return &TareaHandler{repo: r, hub: h}
}

func (h *TareaHandler) Listar(c *gin.Context) {
	eventoID := middleware.GetEventoID(c)
	tareas, err := h.repo.Listar(c.Request.Context(), eventoID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Error listando tareas"})
		return
	}
	c.JSON(http.StatusOK, tareas)
}

func (h *TareaHandler) Completar(c *gin.Context) {
	tareaID := c.Param("id")
	usuarioID := middleware.GetUsuarioID(c)
	eventoID := middleware.GetEventoID(c)
	ctx := c.Request.Context()

	tarea, err := h.repo.Completar(ctx, tareaID, usuarioID)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	go h.hub.PublicarEvento(ctx, eventoID, models.EventoWS{
		Tipo:     models.WSTareaActualizada,
		Payload:  tarea,
		EventoID: eventoID,
	})

	c.JSON(http.StatusOK, tarea)
}

// ─── Chat Handler ─────────────────────────────────────────────────────────────

type ChatHandler struct {
	repo        *repository.MensajeRepository
	usuarioRepo *repository.UsuarioRepository
	hub         *ws.Hub
}

func NewChatHandler(r *repository.MensajeRepository, ur *repository.UsuarioRepository, h *ws.Hub) *ChatHandler {
	return &ChatHandler{repo: r, usuarioRepo: ur, hub: h}
}

func (h *ChatHandler) Historial(c *gin.Context) {
	eventoID := middleware.GetEventoID(c)
	mensajes, err := h.repo.Listar(c.Request.Context(), eventoID, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Error obteniendo historial"})
		return
	}
	c.JSON(http.StatusOK, mensajes)
}

func (h *ChatHandler) Enviar(c *gin.Context) {
	var req models.EnviarMensajeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	usuarioID := middleware.GetUsuarioID(c)
	eventoID := middleware.GetEventoID(c)
	ctx := c.Request.Context()

	// El repo ya trae nombre + rol en el mismo query (WITH inserted + JOIN)
	msg, err := h.repo.Crear(ctx, &models.Mensaje{
		EventoID:  eventoID,
		UsuarioID: usuarioID,
		Contenido: req.Contenido,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Error enviando mensaje"})
		return
	}

	go h.hub.PublicarEvento(ctx, eventoID, models.EventoWS{
		Tipo:     models.WSMensajeNuevo,
		Payload:  msg,
		EventoID: eventoID,
	})

	c.JSON(http.StatusCreated, msg)
}

// ─── WebSocket Handler ────────────────────────────────────────────────────────

type WSHandler struct {
	hub    *ws.Hub
	jwtSvc *auth.JWTService
}

func NewWSHandler(h *ws.Hub, j *auth.JWTService) *WSHandler {
	return &WSHandler{hub: h, jwtSvc: j}
}

// Conectar maneja el upgrade HTTP→WebSocket.
// El token se pasa como query param porque WS no soporta headers custom en algunos clientes.
func (h *WSHandler) Conectar(c *gin.Context) {
	tokenStr := c.Query("token")
	if tokenStr == "" {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Token requerido"})
		return
	}

	claims, err := h.jwtSvc.ValidarToken(tokenStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Token inválido"})
		return
	}

	h.hub.HandleConexion(c.Writer, c.Request, claims.UsuarioID, claims.EventoID)
}
