package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/eventpulse/backend/internal/models"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
)

// ─── Usuario Repository ───────────────────────────────────────────────────────

type UsuarioRepository struct {
	db *sqlx.DB
}

func NewUsuarioRepository(db *sqlx.DB) *UsuarioRepository {
	return &UsuarioRepository{db: db}
}

func (r *UsuarioRepository) BuscarPorEmail(ctx context.Context, email string) (*models.Usuario, error) {
	var u models.Usuario
	err := r.db.GetContext(ctx, &u, `
		SELECT id, evento_id, zona_id, nombre, email, password_hash, rol, activo, creado_en
		FROM usuarios WHERE email = $1 AND activo = true
	`, email)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &u, err
}

func (r *UsuarioRepository) BuscarPorID(ctx context.Context, id string) (*models.Usuario, error) {
	var u models.Usuario
	err := r.db.GetContext(ctx, &u, `
		SELECT id, evento_id, zona_id, nombre, email, password_hash, rol, activo, creado_en
		FROM usuarios WHERE id = $1
	`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &u, err
}

func (r *UsuarioRepository) ValidarPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// ─── Incidencia Repository ────────────────────────────────────────────────────

type IncidenciaRepository struct {
	db *sqlx.DB
}

func NewIncidenciaRepository(db *sqlx.DB) *IncidenciaRepository {
	return &IncidenciaRepository{db: db}
}

func (r *IncidenciaRepository) Listar(ctx context.Context, eventoID string) ([]models.Incidencia, error) {
	var items []models.Incidencia
	err := r.db.SelectContext(ctx, &items, `
		SELECT i.id, i.evento_id, i.zona_id, z.nombre as zona_nombre,
		       i.tipo, i.descripcion, i.estado, i.reportada_por,
		       i.atendida_por, i.creada_en, i.actualizada_en
		FROM incidencias i
		JOIN zonas z ON z.id = i.zona_id
		WHERE i.evento_id = $1
		ORDER BY i.creada_en DESC
	`, eventoID)
	return items, err
}

func (r *IncidenciaRepository) Crear(ctx context.Context, inc *models.Incidencia) (*models.Incidencia, error) {
	var creada models.Incidencia
	err := r.db.GetContext(ctx, &creada, `
		INSERT INTO incidencias (evento_id, zona_id, tipo, descripcion, estado, reportada_por)
		VALUES ($1, $2, $3, $4, 'pendiente', $5)
		RETURNING id, evento_id, zona_id, tipo, descripcion, estado,
		          reportada_por, atendida_por, creada_en, actualizada_en
	`, inc.EventoID, inc.ZonaID, inc.Tipo, inc.Descripcion, inc.ReportadaPor)
	return &creada, err
}

// AtenderConLock usa SELECT FOR UPDATE SKIP LOCKED para manejar concurrencia.
// Si otro usuario ya tomó la incidencia, retorna un error descriptivo en lugar de bloquear.
func (r *IncidenciaRepository) AtenderConLock(ctx context.Context, incID, usuarioID string) (*models.Incidencia, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Intentar tomar el lock de la fila
	var inc models.Incidencia
	err = tx.GetContext(ctx, &inc, `
		SELECT id, estado, atendida_por
		FROM incidencias
		WHERE id = $1
		FOR UPDATE SKIP LOCKED
	`, incID)

	if errors.Is(err, sql.ErrNoRows) {
		// SKIP LOCKED retornó vacío: otro usuario ya tiene el lock
		// Consultamos sin lock para saber quién la tiene
		var actual models.Incidencia
		if dbErr := r.db.GetContext(ctx, &actual, `
			SELECT i.id, i.estado, i.atendida_por, u.nombre as zona_nombre
			FROM incidencias i
			LEFT JOIN usuarios u ON u.id = i.atendida_por
			WHERE i.id = $1
		`, incID); dbErr != nil {
			return nil, fmt.Errorf("incidencia_conflicto:desconocido")
		}
		nombre := "otro usuario"
		if actual.AtendidaPor != nil {
			nombre = actual.ZonaNombre // reutilizamos el campo como nombre del usuario
		}
		return nil, fmt.Errorf("incidencia_conflicto:%s", nombre)
	}
	if err != nil {
		return nil, err
	}

	if inc.Estado != models.EstadoPendiente {
		return nil, fmt.Errorf("incidencia_conflicto:ya está en estado %s", inc.Estado)
	}

	// Actualizar estado
	var actualizada models.Incidencia
	err = tx.GetContext(ctx, &actualizada, `
		UPDATE incidencias
		SET estado = 'en_atencion', atendida_por = $1
		WHERE id = $2
		RETURNING id, evento_id, zona_id, tipo, descripcion, estado,
		          reportada_por, atendida_por, creada_en, actualizada_en
	`, usuarioID, incID)
	if err != nil {
		return nil, err
	}

	// Registrar en historial
	tx.ExecContext(ctx, `
		INSERT INTO incidencias_historial (incidencia_id, estado_anterior, estado_nuevo, usuario_id)
		VALUES ($1, 'pendiente', 'en_atencion', $2)
	`, incID, usuarioID)

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &actualizada, nil
}

func (r *IncidenciaRepository) Resolver(ctx context.Context, incID, usuarioID string) (*models.Incidencia, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var actualizada models.Incidencia
	err = tx.GetContext(ctx, &actualizada, `
		UPDATE incidencias
		SET estado = 'resuelta', atendida_por = $1
		WHERE id = $2 AND (atendida_por = $1 OR $3 = 'supervisor' OR $3 = 'admin')
		RETURNING id, evento_id, zona_id, tipo, descripcion, estado,
		          reportada_por, atendida_por, creada_en, actualizada_en
	`, usuarioID, incID, usuarioID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("no tienes permiso para resolver esta incidencia")
	}
	if err != nil {
		return nil, err
	}

	tx.ExecContext(ctx, `
		INSERT INTO incidencias_historial (incidencia_id, estado_anterior, estado_nuevo, usuario_id)
		VALUES ($1, 'en_atencion', 'resuelta', $2)
	`, incID, usuarioID)

	tx.Commit()
	return &actualizada, nil
}

// ─── Tarea Repository ─────────────────────────────────────────────────────────

type TareaRepository struct {
	db *sqlx.DB
}

func NewTareaRepository(db *sqlx.DB) *TareaRepository {
	return &TareaRepository{db: db}
}

func (r *TareaRepository) Listar(ctx context.Context, eventoID string) ([]models.Tarea, error) {
	var tareas []models.Tarea
	err := r.db.SelectContext(ctx, &tareas, `
		SELECT id, evento_id, zona_id, titulo, descripcion, estado,
		       prioridad, asignada_a, completada_en, creada_en
		FROM tareas
		WHERE evento_id = $1
		ORDER BY
			CASE prioridad WHEN 'alta' THEN 1 WHEN 'media' THEN 2 ELSE 3 END,
			creada_en DESC
	`, eventoID)
	return tareas, err
}

func (r *TareaRepository) Completar(ctx context.Context, tareaID, usuarioID string) (*models.Tarea, error) {
	ahora := time.Now()
	var tarea models.Tarea
	err := r.db.GetContext(ctx, &tarea, `
		UPDATE tareas
		SET estado = 'completada', asignada_a = $1, completada_en = $2
		WHERE id = $3 AND estado = 'pendiente'
		RETURNING id, evento_id, zona_id, titulo, descripcion, estado,
		          prioridad, asignada_a, completada_en, creada_en
	`, usuarioID, ahora, tareaID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("tarea no encontrada o ya completada")
	}
	return &tarea, err
}

// ─── Mensaje Repository ───────────────────────────────────────────────────────

type MensajeRepository struct {
	db *sqlx.DB
}

func NewMensajeRepository(db *sqlx.DB) *MensajeRepository {
	return &MensajeRepository{db: db}
}

func (r *MensajeRepository) Listar(ctx context.Context, eventoID string, limite int) ([]models.Mensaje, error) {
	var mensajes []models.Mensaje
	err := r.db.SelectContext(ctx, &mensajes, `
		SELECT m.id, m.evento_id, m.usuario_id,
		       u.nombre    AS nombre_usuario,
		       u.rol       AS rol_usuario,
		       m.contenido, m.enviado_en
		FROM mensajes m
		JOIN usuarios u ON u.id = m.usuario_id
		WHERE m.evento_id = $1
		ORDER BY m.enviado_en DESC
		LIMIT $2
	`, eventoID, limite)
	// Revertir para orden cronológico
	for i, j := 0, len(mensajes)-1; i < j; i, j = i+1, j-1 {
		mensajes[i], mensajes[j] = mensajes[j], mensajes[i]
	}
	return mensajes, err
}

func (r *MensajeRepository) Crear(ctx context.Context, msg *models.Mensaje) (*models.Mensaje, error) {
	// Insertar y luego traer el nombre + rol en un solo query
	var creado models.Mensaje
	err := r.db.GetContext(ctx, &creado, `
		WITH inserted AS (
			INSERT INTO mensajes (evento_id, usuario_id, contenido)
			VALUES ($1, $2, $3)
			RETURNING id, evento_id, usuario_id, contenido, enviado_en
		)
		SELECT i.id, i.evento_id, i.usuario_id,
		       u.nombre AS nombre_usuario,
		       u.rol    AS rol_usuario,
		       i.contenido, i.enviado_en
		FROM inserted i
		JOIN usuarios u ON u.id = i.usuario_id
	`, msg.EventoID, msg.UsuarioID, msg.Contenido)
	return &creado, err
}
