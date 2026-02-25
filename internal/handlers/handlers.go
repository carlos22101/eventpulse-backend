package handlers

import (
	"net/http"
	"strings"

	"github.com/eventpulse/backend/internal/auth"
	"github.com/eventpulse/backend/internal/middleware"
	"github.com/eventpulse/backend/internal/models"
	"github.com/eventpulse/backend/internal/repository"
	"github.com/eventpulse/backend/internal/ws"
	"github.com/gin-gonic/gin"
)

// ─── Auth ─────────────────────────────────────────────────────────────────────

type AuthHandler struct {
	usuarioRepo *repository.UsuarioRepo
	eventoRepo  *repository.EventoRepo
	jwtSvc      *auth.JWTService
}

func NewAuthHandler(u *repository.UsuarioRepo, e *repository.EventoRepo, j *auth.JWTService) *AuthHandler {
	return &AuthHandler{usuarioRepo: u, eventoRepo: e, jwtSvc: j}
}

// POST /api/v1/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	usuario, err := h.usuarioRepo.BuscarPorNombreUsuario(c.Request.Context(), req.NombreUsuario)
	if err != nil || usuario == nil || !h.usuarioRepo.ValidarPassword(usuario.Password, req.Password) {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Credenciales inválidas"})
		return
	}
	token, err := h.jwtSvc.GenerarToken(usuario)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Error generando token"})
		return
	}
	usuario.Password = ""
	c.JSON(http.StatusOK, models.LoginResponse{Token: token, Usuario: *usuario})
}

// GET /api/v1/auth/me
func (h *AuthHandler) Me(c *gin.Context) {
	u, err := h.usuarioRepo.BuscarPorID(c.Request.Context(), middleware.GetUsuarioID(c))
	if err != nil || u == nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Usuario no encontrado"})
		return
	}
	u.Password = ""
	c.JSON(http.StatusOK, u)
}

// ─── Evento ───────────────────────────────────────────────────────────────────

type EventoHandler struct {
	eventoRepo  *repository.EventoRepo
	usuarioRepo *repository.UsuarioRepo
	hub         *ws.Hub
}

func NewEventoHandler(e *repository.EventoRepo, u *repository.UsuarioRepo, h *ws.Hub) *EventoHandler {
	return &EventoHandler{eventoRepo: e, usuarioRepo: u, hub: h}
}

// GET /api/v1/eventos  (admin: todos | trabajador: solo el activo vinculado)
func (h *EventoHandler) Listar(c *gin.Context) {
	ctx := c.Request.Context()
	if middleware.GetRol(c) == models.RolAdmin {
		lista, err := h.eventoRepo.Listar(ctx)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Error listando eventos"})
			return
		}
		c.JSON(http.StatusOK, lista)
		return
	}
	// Trabajador: devolver solo el evento al que está vinculado
	eventoID := middleware.GetEventoID(c)
	if eventoID == "" {
		c.JSON(http.StatusOK, []models.Evento{})
		return
	}
	evento, err := h.eventoRepo.ObtenerActivo(ctx)
	if err != nil || evento == nil {
		c.JSON(http.StatusOK, []models.Evento{})
		return
	}
	c.JSON(http.StatusOK, []models.Evento{*evento})
}

// POST /api/v1/eventos  [solo admin]
func (h *EventoHandler) Crear(c *gin.Context) {
	var req models.CrearEventoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	evento, err := h.eventoRepo.Crear(c.Request.Context(), &req, middleware.GetUsuarioID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Error creando evento"})
		return
	}
	c.JSON(http.StatusCreated, evento)
}

// PATCH /api/v1/eventos/:id/terminar  [solo admin]
func (h *EventoHandler) Terminar(c *gin.Context) {
	eventoID := c.Param("id")
	ctx := c.Request.Context()

	evento, err := h.eventoRepo.Terminar(ctx, eventoID)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	// Notificar a todos los conectados que el evento terminó
	go h.hub.Publicar(ctx, eventoID, models.EventoWS{
		Tipo:     models.WSEventoTerminado,
		Payload:  evento,
		EventoID: eventoID,
	})

	c.JSON(http.StatusOK, evento)
}

// ─── Usuario ──────────────────────────────────────────────────────────────────

type UsuarioHandler struct {
	usuarioRepo *repository.UsuarioRepo
	eventoRepo  *repository.EventoRepo
}

func NewUsuarioHandler(u *repository.UsuarioRepo, e *repository.EventoRepo) *UsuarioHandler {
	return &UsuarioHandler{usuarioRepo: u, eventoRepo: e}
}

// POST /api/v1/usuarios  [solo admin]
// Crea el usuario y lo vincula automáticamente al evento activo
func (h *UsuarioHandler) Crear(c *gin.Context) {
	var req models.CrearUsuarioRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	if !req.Rol.EsValido() || req.Rol == models.RolAdmin {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Rol inválido. Válidos: aseo, guardia, medico, logistica, supervisor",
		})
		return
	}

	// Obtener evento activo para vincular
	evento, err := h.eventoRepo.ObtenerActivo(c.Request.Context())
	if err != nil || evento == nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "No hay ningún evento activo. Crea un evento primero"})
		return
	}

	usuario, err := h.usuarioRepo.Crear(c.Request.Context(), &req, evento.ID)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			c.JSON(http.StatusConflict, models.ErrorResponse{Error: "El nombre de usuario ya existe"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Error creando usuario"})
		return
	}
	c.JSON(http.StatusCreated, usuario)
}

// GET /api/v1/usuarios  [solo admin]
func (h *UsuarioHandler) Listar(c *gin.Context) {
	eventoID := middleware.GetEventoID(c)
	// Admin puede pasar ?evento_id= para ver de otro evento
	if qID := c.Query("evento_id"); qID != "" && middleware.GetRol(c) == models.RolAdmin {
		eventoID = qID
	}
	if eventoID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Se requiere evento_id"})
		return
	}
	lista, err := h.usuarioRepo.Listar(c.Request.Context(), eventoID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Error listando usuarios"})
		return
	}
	c.JSON(http.StatusOK, lista)
}

// ─── Zona ─────────────────────────────────────────────────────────────────────

type ZonaHandler struct {
	zonaRepo   *repository.ZonaRepo
	eventoRepo *repository.EventoRepo
}

func NewZonaHandler(z *repository.ZonaRepo, e *repository.EventoRepo) *ZonaHandler {
	return &ZonaHandler{zonaRepo: z, eventoRepo: e}
}

// GET /api/v1/zonas
func (h *ZonaHandler) Listar(c *gin.Context) {
	eventoID := middleware.GetEventoID(c)
	if qID := c.Query("evento_id"); qID != "" && middleware.GetRol(c) == models.RolAdmin {
		eventoID = qID
	}
	lista, err := h.zonaRepo.Listar(c.Request.Context(), eventoID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Error listando zonas"})
		return
	}
	c.JSON(http.StatusOK, lista)
}

// POST /api/v1/zonas  [solo admin]
func (h *ZonaHandler) Crear(c *gin.Context) {
	var req models.CrearZonaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	// Obtener evento activo
	evento, err := h.eventoRepo.ObtenerActivo(c.Request.Context())
	if err != nil || evento == nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "No hay evento activo"})
		return
	}
	zona, err := h.zonaRepo.Crear(c.Request.Context(), &req, evento.ID)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			c.JSON(http.StatusConflict, models.ErrorResponse{Error: "Ya existe una zona con ese ID en este evento"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Error creando zona"})
		return
	}
	c.JSON(http.StatusCreated, zona)
}

// DELETE /api/v1/zonas/:id  [solo admin]
func (h *ZonaHandler) Eliminar(c *gin.Context) {
	zonaID := c.Param("id")
	evento, err := h.eventoRepo.ObtenerActivo(c.Request.Context())
	if err != nil || evento == nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "No hay evento activo"})
		return
	}
	if err := h.zonaRepo.Eliminar(c.Request.Context(), zonaID, evento.ID); err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"mensaje": "Zona eliminada"})
}

// ─── Incidencia ───────────────────────────────────────────────────────────────

type IncidenciaHandler struct {
	repo       *repository.IncidenciaRepo
	eventoRepo *repository.EventoRepo
	hub        *ws.Hub
}

func NewIncidenciaHandler(r *repository.IncidenciaRepo, e *repository.EventoRepo, h *ws.Hub) *IncidenciaHandler {
	return &IncidenciaHandler{repo: r, eventoRepo: e, hub: h}
}

// GET /api/v1/incidencias
func (h *IncidenciaHandler) Listar(c *gin.Context) {
	eventoID := middleware.GetEventoID(c)
	if eventoID == "" {
		if ev, _ := h.eventoRepo.ObtenerActivo(c.Request.Context()); ev != nil {
			eventoID = ev.ID
		}
	}
	lista, err := h.repo.Listar(c.Request.Context(), eventoID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Error listando incidencias"})
		return
	}
	c.JSON(http.StatusOK, lista)
}

// GET /api/v1/incidencias/:id
func (h *IncidenciaHandler) ObtenerPorID(c *gin.Context) {
	inc, err := h.repo.ObtenerPorID(c.Request.Context(), c.Param("id"))
	if err != nil || inc == nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Incidencia no encontrada"})
		return
	}
	c.JSON(http.StatusOK, inc)
}

// POST /api/v1/incidencias  [solo admin]
func (h *IncidenciaHandler) Crear(c *gin.Context) {
	var req models.CrearIncidenciaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	ctx := c.Request.Context()
	evento, err := h.eventoRepo.ObtenerActivo(ctx)
	if err != nil || evento == nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "No hay evento activo"})
		return
	}
	inc, err := h.repo.Crear(ctx, &req, evento.ID, middleware.GetUsuarioID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Error creando incidencia"})
		return
	}
	// Notificar a todos por WS
	go h.hub.Publicar(ctx, evento.ID, models.EventoWS{
		Tipo:     models.WSIncidenciaNueva,
		Payload:  inc,
		EventoID: evento.ID,
	})
	c.JSON(http.StatusCreated, inc)
}

// PATCH /api/v1/incidencias/:id  (trabajador cambia estado | admin puede reasignar)
func (h *IncidenciaHandler) Editar(c *gin.Context) {
	var req models.EditarIncidenciaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	ctx := c.Request.Context()
	inc, err := h.repo.Editar(ctx, c.Param("id"), &req, middleware.GetUsuarioID(c))
	if err != nil || inc == nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Error editando incidencia"})
		return
	}
	// Notificar cambio a todos
	go h.hub.Publicar(ctx, inc.EventoID, models.EventoWS{
		Tipo:     models.WSIncidenciaActualizada,
		Payload:  inc,
		EventoID: inc.EventoID,
	})
	c.JSON(http.StatusOK, inc)
}

// ─── Tarea ────────────────────────────────────────────────────────────────────

type TareaHandler struct {
	repo       *repository.TareaRepo
	eventoRepo *repository.EventoRepo
	hub        *ws.Hub
}

func NewTareaHandler(r *repository.TareaRepo, e *repository.EventoRepo, h *ws.Hub) *TareaHandler {
	return &TareaHandler{repo: r, eventoRepo: e, hub: h}
}

// GET /api/v1/tareas
func (h *TareaHandler) Listar(c *gin.Context) {
	eventoID := middleware.GetEventoID(c)
	if eventoID == "" {
		if ev, _ := h.eventoRepo.ObtenerActivo(c.Request.Context()); ev != nil {
			eventoID = ev.ID
		}
	}
	lista, err := h.repo.Listar(c.Request.Context(), eventoID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Error listando tareas"})
		return
	}
	c.JSON(http.StatusOK, lista)
}

// GET /api/v1/tareas/:id
func (h *TareaHandler) ObtenerPorID(c *gin.Context) {
	t, err := h.repo.ObtenerPorID(c.Request.Context(), c.Param("id"))
	if err != nil || t == nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Tarea no encontrada"})
		return
	}
	c.JSON(http.StatusOK, t)
}

// POST /api/v1/tareas  [solo admin]
func (h *TareaHandler) Crear(c *gin.Context) {
	var req models.CrearTareaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	ctx := c.Request.Context()
	evento, err := h.eventoRepo.ObtenerActivo(ctx)
	if err != nil || evento == nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "No hay evento activo"})
		return
	}
	tarea, err := h.repo.Crear(ctx, &req, evento.ID, middleware.GetUsuarioID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Error creando tarea"})
		return
	}
	go h.hub.Publicar(ctx, evento.ID, models.EventoWS{
		Tipo:     models.WSTareaNueva,
		Payload:  tarea,
		EventoID: evento.ID,
	})
	c.JSON(http.StatusCreated, tarea)
}

// PATCH /api/v1/tareas/:id  (trabajador marca completada | admin reasigna)
func (h *TareaHandler) Editar(c *gin.Context) {
	var req models.EditarTareaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	ctx := c.Request.Context()
	tarea, err := h.repo.Editar(ctx, c.Param("id"), &req)
	if err != nil || tarea == nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Error editando tarea"})
		return
	}
	go h.hub.Publicar(ctx, tarea.EventoID, models.EventoWS{
		Tipo:     models.WSTareaActualizada,
		Payload:  tarea,
		EventoID: tarea.EventoID,
	})
	c.JSON(http.StatusOK, tarea)
}

// ─── Chat ─────────────────────────────────────────────────────────────────────

type ChatHandler struct {
	mensajeRepo *repository.MensajeRepo
	eventoRepo  *repository.EventoRepo
	hub         *ws.Hub
}

func NewChatHandler(m *repository.MensajeRepo, e *repository.EventoRepo, h *ws.Hub) *ChatHandler {
	return &ChatHandler{mensajeRepo: m, eventoRepo: e, hub: h}
}

// GET /api/v1/chat/historial
func (h *ChatHandler) Historial(c *gin.Context) {
	eventoID := middleware.GetEventoID(c)
	if eventoID == "" {
		if ev, _ := h.eventoRepo.ObtenerActivo(c.Request.Context()); ev != nil {
			eventoID = ev.ID
		}
	}
	msgs, err := h.mensajeRepo.Listar(c.Request.Context(), eventoID, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Error obteniendo historial"})
		return
	}
	c.JSON(http.StatusOK, msgs)
}

// POST /api/v1/chat/mensaje  (admin y trabajadores)
func (h *ChatHandler) Enviar(c *gin.Context) {
	var req models.EnviarMensajeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	ctx := c.Request.Context()
	usuarioID := middleware.GetUsuarioID(c)
	eventoID := middleware.GetEventoID(c)

	// Admin puede enviar aunque no tenga evento_id en token, usar el activo
	if eventoID == "" {
		if ev, _ := h.eventoRepo.ObtenerActivo(ctx); ev != nil {
			eventoID = ev.ID
		}
	}
	if eventoID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "No hay evento activo"})
		return
	}

	msg, err := h.mensajeRepo.Crear(ctx, eventoID, usuarioID, req.Contenido)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Error enviando mensaje"})
		return
	}

	// Distribuir a todos por WS — aquí es donde todos ven nombre + rol
	go h.hub.Publicar(ctx, eventoID, models.EventoWS{
		Tipo:     models.WSMensajeNuevo,
		Payload:  msg,
		EventoID: eventoID,
	})

	c.JSON(http.StatusCreated, msg)
}

// ─── WebSocket ────────────────────────────────────────────────────────────────

type WSHandler struct {
	hub        *ws.Hub
	jwtSvc     *auth.JWTService
	eventoRepo *repository.EventoRepo
}

func NewWSHandler(h *ws.Hub, j *auth.JWTService, e *repository.EventoRepo) *WSHandler {
	return &WSHandler{hub: h, jwtSvc: j, eventoRepo: e}
}

// GET /ws?token=JWT
// Admin se conecta usando el evento_id activo aunque su token no lo tenga
func (h *WSHandler) Conectar(c *gin.Context) {
	tokenStr := c.Query("token")
	if tokenStr == "" {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Token requerido como query param: ?token=..."})
		return
	}
	claims, err := h.jwtSvc.ValidarToken(tokenStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Token inválido"})
		return
	}

	eventoID := claims.EventoID

	// Si es admin y no tiene evento en el token, usar el evento activo
	if (eventoID == nil || *eventoID == "") && claims.Rol == models.RolAdmin {
		if ev, _ := h.eventoRepo.ObtenerActivo(c.Request.Context()); ev != nil {
			eventoID = &ev.ID
		}
	}

	if eventoID == nil || *eventoID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "No hay evento activo al que conectarse"})
		return
	}

	h.hub.HandleConexion(c.Writer, c.Request, claims.UsuarioID, *eventoID)
}
