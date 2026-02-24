package models

import "time"

// ─── Roles ────────────────────────────────────────────────────────────────────

type Rol string

const (
	RolAseo        Rol = "aseo"
	RolGuardia     Rol = "guardia"
	RolMedico      Rol = "medico"
	RolLogistica   Rol = "logistica"
	RolSupervisor  Rol = "supervisor"
	RolAdmin       Rol = "admin"
)

// Etiqueta visual para mostrar en el chat
func (r Rol) Etiqueta() string {
	switch r {
	case RolAseo:
		return "🧹 Aseo"
	case RolGuardia:
		return "🛡️ Guardia"
	case RolMedico:
		return "🏥 Médico"
	case RolLogistica:
		return "📦 Logística"
	case RolSupervisor:
		return "⭐ Supervisor"
	case RolAdmin:
		return "👑 Admin"
	default:
		return string(r)
	}
}

// ─── Usuario ──────────────────────────────────────────────────────────────────

type Usuario struct {
	ID        string    `json:"id" db:"id"`
	Nombre    string    `json:"nombre" db:"nombre"`
	Email     string    `json:"email" db:"email"`
	Password  string    `json:"-" db:"password_hash"`
	Rol       Rol       `json:"rol" db:"rol"`
	ZonaID    *string   `json:"zona_id,omitempty" db:"zona_id"`
	EventoID  string    `json:"evento_id" db:"evento_id"`
	Activo    bool      `json:"activo" db:"activo"`
	CreadoEn  time.Time `json:"creado_en" db:"creado_en"`
}

// ─── Evento ───────────────────────────────────────────────────────────────────

type Evento struct {
	ID          string    `json:"id" db:"id"`
	Nombre      string    `json:"nombre" db:"nombre"`
	Descripcion string    `json:"descripcion" db:"descripcion"`
	FechaInicio time.Time `json:"fecha_inicio" db:"fecha_inicio"`
	FechaFin    time.Time `json:"fecha_fin" db:"fecha_fin"`
	Activo      bool      `json:"activo" db:"activo"`
	CreadoEn    time.Time `json:"creado_en" db:"creado_en"`
}

// ─── Zona ─────────────────────────────────────────────────────────────────────

type Zona struct {
	ID       string  `json:"id" db:"id"`
	EventoID string  `json:"evento_id" db:"evento_id"`
	Nombre   string  `json:"nombre" db:"nombre"`
	PosX     float64 `json:"pos_x" db:"pos_x"` // % relativo al mapa (0-100)
	PosY     float64 `json:"pos_y" db:"pos_y"`
	Color    string  `json:"color" db:"color"`
}

// ─── Incidencia ───────────────────────────────────────────────────────────────

type EstadoIncidencia string

const (
	EstadoPendiente   EstadoIncidencia = "pendiente"
	EstadoEnAtencion  EstadoIncidencia = "en_atencion"
	EstadoResuelta    EstadoIncidencia = "resuelta"
)

type TipoIncidencia string

const (
	TipoDerrame        TipoIncidencia = "derrame"
	TipoSeguridad      TipoIncidencia = "seguridad"
	TipoReabastecimiento TipoIncidencia = "reabastecimiento"
	TipoMedico         TipoIncidencia = "medico"
	TipoOtro           TipoIncidencia = "otro"
)

type Incidencia struct {
	ID           string           `json:"id" db:"id"`
	EventoID     string           `json:"evento_id" db:"evento_id"`
	ZonaID       string           `json:"zona_id" db:"zona_id"`
	ZonaNombre   string           `json:"zona_nombre,omitempty" db:"zona_nombre"`
	Tipo         TipoIncidencia   `json:"tipo" db:"tipo"`
	Descripcion  string           `json:"descripcion" db:"descripcion"`
	Estado       EstadoIncidencia `json:"estado" db:"estado"`
	ReportadaPor string           `json:"reportada_por" db:"reportada_por"`
	AtendidaPor  *string          `json:"atendida_por,omitempty" db:"atendida_por"`
	CreadaEn     time.Time        `json:"creada_en" db:"creada_en"`
	ActualizadaEn time.Time       `json:"actualizada_en" db:"actualizada_en"`
}

// ─── Tarea ────────────────────────────────────────────────────────────────────

type EstadoTarea string

const (
	TareaPendiente   EstadoTarea = "pendiente"
	TareaCompletada  EstadoTarea = "completada"
)

type PrioridadTarea string

const (
	PrioridadAlta   PrioridadTarea = "alta"
	PrioridadMedia  PrioridadTarea = "media"
	PrioridadBaja   PrioridadTarea = "baja"
)

type Tarea struct {
	ID           string         `json:"id" db:"id"`
	EventoID     string         `json:"evento_id" db:"evento_id"`
	ZonaID       *string        `json:"zona_id,omitempty" db:"zona_id"`
	Titulo       string         `json:"titulo" db:"titulo"`
	Descripcion  string         `json:"descripcion" db:"descripcion"`
	Estado       EstadoTarea    `json:"estado" db:"estado"`
	Prioridad    PrioridadTarea `json:"prioridad" db:"prioridad"`
	AsignadaA    *string        `json:"asignada_a,omitempty" db:"asignada_a"`
	CompletadaEn *time.Time     `json:"completada_en,omitempty" db:"completada_en"`
	CreadaEn     time.Time      `json:"creada_en" db:"creada_en"`
}

// ─── Mensaje Chat ─────────────────────────────────────────────────────────────

type Mensaje struct {
	ID          string    `json:"id" db:"id"`
	EventoID    string    `json:"evento_id" db:"evento_id"`
	UsuarioID   string    `json:"usuario_id" db:"usuario_id"`
	NombreUser  string    `json:"nombre_usuario,omitempty" db:"nombre_usuario"`
	RolUsuario  Rol       `json:"rol_usuario,omitempty" db:"rol_usuario"`   // ← nuevo
	Contenido   string    `json:"contenido" db:"contenido"`
	EnviadoEn   time.Time `json:"enviado_en" db:"enviado_en"`
}

// ─── DTOs de Request/Response ─────────────────────────────────────────────────

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type LoginResponse struct {
	Token   string  `json:"token"`
	Usuario Usuario `json:"usuario"`
}

type ReportarIncidenciaRequest struct {
	ZonaID      string         `json:"zona_id" binding:"required"`
	Tipo        TipoIncidencia `json:"tipo" binding:"required"`
	Descripcion string         `json:"descripcion" binding:"required,min=5"`
}

type AtenderIncidenciaRequest struct {
	IncidenciaID string `json:"incidencia_id" binding:"required"`
}

type CompletarTareaRequest struct {
	TareaID string `json:"tarea_id" binding:"required"`
}

type EnviarMensajeRequest struct {
	Contenido string `json:"contenido" binding:"required,min=1,max=500"`
}

// ─── Eventos WebSocket ────────────────────────────────────────────────────────

type TipoEventoWS string

const (
	WSIncidenciaNueva      TipoEventoWS = "incidencia_nueva"
	WSIncidenciaActualizada TipoEventoWS = "incidencia_actualizada"
	WSIncidenciaConflicto  TipoEventoWS = "incidencia_conflicto"   // Rollback
	WSTareaActualizada     TipoEventoWS = "tarea_actualizada"
	WSMensajeNuevo         TipoEventoWS = "mensaje_nuevo"
	WSAlertaEmergencia     TipoEventoWS = "alerta_emergencia"
	WSPing                 TipoEventoWS = "ping"
)

type EventoWS struct {
	Tipo     TipoEventoWS `json:"tipo"`
	Payload  interface{}  `json:"payload"`
	EventoID string       `json:"evento_id"`
}

type ConflictoPayload struct {
	IncidenciaID string `json:"incidencia_id"`
	Mensaje      string `json:"mensaje"`
	AtendidaPor  string `json:"atendida_por"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Codigo  int    `json:"codigo,omitempty"`
}
