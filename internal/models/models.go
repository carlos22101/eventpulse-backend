package models

import "time"

// ─── Roles ────────────────────────────────────────────────────────────────────

type Rol string

const (
	RolAdmin      Rol = "admin"
	RolAseo       Rol = "aseo"
	RolGuardia    Rol = "guardia"
	RolMedico     Rol = "medico"
	RolLogistica  Rol = "logistica"
	RolSupervisor Rol = "supervisor"
)

func (r Rol) EsValido() bool {
	switch r {
	case RolAdmin, RolAseo, RolGuardia, RolMedico, RolLogistica, RolSupervisor:
		return true
	}
	return false
}

func (r Rol) Etiqueta() string {
	switch r {
	case RolAdmin:
		return "👑 Admin"
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
	default:
		return string(r)
	}
}

// ─── Evento ───────────────────────────────────────────────────────────────────

type EstadoEvento string

const (
	EventoActivo    EstadoEvento = "activo"
	EventoTerminado EstadoEvento = "terminado"
)

type Evento struct {
	ID          string       `json:"id" db:"id"`
	Nombre      string       `json:"nombre" db:"nombre"`
	Descripcion string       `json:"descripcion" db:"descripcion"`
	Estado      EstadoEvento `json:"estado" db:"estado"`
	CreadoPor   string       `json:"creado_por" db:"creado_por"`
	CreadoEn    time.Time    `json:"creado_en" db:"creado_en"`
	TerminadoEn *time.Time   `json:"terminado_en,omitempty" db:"terminado_en"`
}

// ─── Zona ─────────────────────────────────────────────────────────────────────

type Zona struct {
	ID       string `json:"id" db:"id"`             // ID manual ej: "bano-norte", "pasillo-4"
	EventoID string `json:"evento_id" db:"evento_id"`
	Nombre   string `json:"nombre" db:"nombre"`
}

// ─── Usuario ──────────────────────────────────────────────────────────────────

type Usuario struct {
	ID           string     `json:"id" db:"id"`
	NombreUsuario string    `json:"nombre_usuario" db:"nombre_usuario"` // username de login
	Nombre       string     `json:"nombre" db:"nombre"`                 // nombre visible
	Password     string     `json:"-" db:"password_hash"`
	Rol          Rol        `json:"rol" db:"rol"`
	EventoID     *string    `json:"evento_id,omitempty" db:"evento_id"` // vinculado al evento activo
	Activo       bool       `json:"activo" db:"activo"`
	CreadoEn     time.Time  `json:"creado_en" db:"creado_en"`
}

// ─── Incidencia ───────────────────────────────────────────────────────────────

type EstadoIncidencia string

const (
	IncidenciaPendiente  EstadoIncidencia = "pendiente"
	IncidenciaEnAtencion EstadoIncidencia = "en_atencion"
	IncidenciaResuelta   EstadoIncidencia = "resuelta"
)

type TipoIncidencia string

const (
	TipoDerrame          TipoIncidencia = "derrame"
	TipoSeguridad        TipoIncidencia = "seguridad"
	TipoReabastecimiento TipoIncidencia = "reabastecimiento"
	TipoMedico           TipoIncidencia = "medico"
	TipoOtro             TipoIncidencia = "otro"
)

type Incidencia struct {
	ID            string           `json:"id" db:"id"`
	EventoID      string           `json:"evento_id" db:"evento_id"`
	ZonaID        string           `json:"zona_id" db:"zona_id"`
	ZonaNombre    string           `json:"zona_nombre,omitempty" db:"zona_nombre"`
	Tipo          TipoIncidencia   `json:"tipo" db:"tipo"`
	Descripcion   string           `json:"descripcion" db:"descripcion"`
	Estado        EstadoIncidencia `json:"estado" db:"estado"`
	CreadaPor     string           `json:"creada_por" db:"creada_por"`          // admin
	AsignadaA     *string          `json:"asignada_a,omitempty" db:"asignada_a"` // trabajador que la atiende
	NombreAsignado *string         `json:"nombre_asignado,omitempty" db:"nombre_asignado"`
	CreadaEn      time.Time        `json:"creada_en" db:"creada_en"`
	ActualizadaEn time.Time        `json:"actualizada_en" db:"actualizada_en"`
}

// ─── Tarea ────────────────────────────────────────────────────────────────────

type EstadoTarea string

const (
	TareaPendiente  EstadoTarea = "pendiente"
	TareaEnProgreso EstadoTarea = "en_progreso"
	TareaCompletada EstadoTarea = "completada"
)

type PrioridadTarea string

const (
	PrioridadAlta  PrioridadTarea = "alta"
	PrioridadMedia PrioridadTarea = "media"
	PrioridadBaja  PrioridadTarea = "baja"
)

type Tarea struct {
	ID             string         `json:"id" db:"id"`
	EventoID       string         `json:"evento_id" db:"evento_id"`
	ZonaID         *string        `json:"zona_id,omitempty" db:"zona_id"`
	ZonaNombre     *string        `json:"zona_nombre,omitempty" db:"zona_nombre"`
	Titulo         string         `json:"titulo" db:"titulo"`
	Descripcion    string         `json:"descripcion" db:"descripcion"`
	Estado         EstadoTarea    `json:"estado" db:"estado"`
	Prioridad      PrioridadTarea `json:"prioridad" db:"prioridad"`
	CreadaPor      string         `json:"creada_por" db:"creada_por"`
	AsignadaA      *string        `json:"asignada_a,omitempty" db:"asignada_a"`
	NombreAsignado *string        `json:"nombre_asignado,omitempty" db:"nombre_asignado"`
	CompletadaEn   *time.Time     `json:"completada_en,omitempty" db:"completada_en"`
	CreadaEn       time.Time      `json:"creada_en" db:"creada_en"`
}

// ─── Mensaje Chat ─────────────────────────────────────────────────────────────

type Mensaje struct {
	ID            string    `json:"id" db:"id"`
	EventoID      string    `json:"evento_id" db:"evento_id"`
	UsuarioID     string    `json:"usuario_id" db:"usuario_id"`
	NombreUsuario string    `json:"nombre_usuario" db:"nombre_usuario"`
	RolUsuario    Rol       `json:"rol_usuario" db:"rol_usuario"`
	Contenido     string    `json:"contenido" db:"contenido"`
	EnviadoEn     time.Time `json:"enviado_en" db:"enviado_en"`
}

// ─── Eventos WebSocket ────────────────────────────────────────────────────────

type TipoEventoWS string

const (
	// Incidencias
	WSIncidenciaNueva       TipoEventoWS = "incidencia_nueva"
	WSIncidenciaActualizada TipoEventoWS = "incidencia_actualizada"
	// Tareas
	WSTareaNueva       TipoEventoWS = "tarea_nueva"
	WSTareaActualizada TipoEventoWS = "tarea_actualizada"
	// Chat grupal
	WSMensajeNuevo TipoEventoWS = "mensaje_nuevo"
	// Sistema
	WSEventoTerminado TipoEventoWS = "evento_terminado"
	WSPing            TipoEventoWS = "ping"
)

type EventoWS struct {
	Tipo     TipoEventoWS `json:"tipo"`
	Payload  interface{}  `json:"payload"`
	EventoID string       `json:"evento_id"`
}

// ─── DTOs Request ─────────────────────────────────────────────────────────────

type LoginRequest struct {
	NombreUsuario string `json:"nombre_usuario" binding:"required"`
	Password      string `json:"password" binding:"required,min=4"`
}

type LoginResponse struct {
	Token   string  `json:"token"`
	Usuario Usuario `json:"usuario"`
}

type CrearEventoRequest struct {
	Nombre      string `json:"nombre" binding:"required,min=3"`
	Descripcion string `json:"descripcion"`
}

type TerminarEventoRequest struct {
	EventoID string `json:"evento_id" binding:"required"`
}

type CrearZonaRequest struct {
	ID     string `json:"id" binding:"required"`     // manual ej: "bano-norte"
	Nombre string `json:"nombre" binding:"required"`
}

type CrearUsuarioRequest struct {
	NombreUsuario string  `json:"nombre_usuario" binding:"required,min=3"`
	Nombre        string  `json:"nombre" binding:"required,min=2"`
	Password      string  `json:"password" binding:"required,min=4"`
	Rol           Rol     `json:"rol" binding:"required"`
	ZonaID        *string `json:"zona_id,omitempty"`
}

type CrearIncidenciaRequest struct {
	ZonaID      string         `json:"zona_id" binding:"required"`
	Tipo        TipoIncidencia `json:"tipo" binding:"required"`
	Descripcion string         `json:"descripcion" binding:"required,min=5"`
	AsignadaA   *string        `json:"asignada_a,omitempty"`
}

type EditarIncidenciaRequest struct {
	Estado    *EstadoIncidencia `json:"estado,omitempty"`
	AsignadaA *string           `json:"asignada_a,omitempty"`
}

type CrearTareaRequest struct {
	ZonaID      string         `json:"zona_id,omitempty"`
	Titulo      string         `json:"titulo" binding:"required,min=3"`
	Descripcion string         `json:"descripcion"`
	Prioridad   PrioridadTarea `json:"prioridad" binding:"required"`
	AsignadaA   *string        `json:"asignada_a,omitempty"`
}

type EditarTareaRequest struct {
	Estado    *EstadoTarea `json:"estado,omitempty"`
	AsignadaA *string      `json:"asignada_a,omitempty"`
}

type EnviarMensajeRequest struct {
	Contenido string `json:"contenido" binding:"required,min=1,max=500"`
}

type ErrorResponse struct {
	Error  string `json:"error"`
	Codigo int    `json:"codigo,omitempty"`
}
