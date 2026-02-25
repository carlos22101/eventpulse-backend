package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/eventpulse/backend/internal/models"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
)

// ─── Usuario ──────────────────────────────────────────────────────────────────

type UsuarioRepo struct{ db *sqlx.DB }

func NewUsuarioRepo(db *sqlx.DB) *UsuarioRepo { return &UsuarioRepo{db: db} }

func (r *UsuarioRepo) BuscarPorNombreUsuario(ctx context.Context, nombreUsuario string) (*models.Usuario, error) {
	var u models.Usuario
	err := r.db.GetContext(ctx, &u, `
		SELECT id, nombre_usuario, nombre, password_hash, rol, evento_id, activo, creado_en
		FROM usuarios WHERE nombre_usuario = $1 AND activo = true
	`, nombreUsuario)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &u, err
}

func (r *UsuarioRepo) BuscarPorID(ctx context.Context, id string) (*models.Usuario, error) {
	var u models.Usuario
	err := r.db.GetContext(ctx, &u, `
		SELECT id, nombre_usuario, nombre, password_hash, rol, evento_id, activo, creado_en
		FROM usuarios WHERE id = $1
	`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &u, err
}

func (r *UsuarioRepo) Crear(ctx context.Context, req *models.CrearUsuarioRequest, eventoID string) (*models.Usuario, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	var u models.Usuario
	err = r.db.GetContext(ctx, &u, `
		INSERT INTO usuarios (nombre_usuario, nombre, password_hash, rol, evento_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, nombre_usuario, nombre, rol, evento_id, activo, creado_en
	`, req.NombreUsuario, req.Nombre, string(hash), req.Rol, eventoID)
	return &u, err
}

func (r *UsuarioRepo) Listar(ctx context.Context, eventoID string) ([]models.Usuario, error) {
	var lista []models.Usuario
	err := r.db.SelectContext(ctx, &lista, `
		SELECT id, nombre_usuario, nombre, rol, evento_id, activo, creado_en
		FROM usuarios
		WHERE evento_id = $1 AND activo = true
		ORDER BY rol, nombre
	`, eventoID)
	return lista, err
}

func (r *UsuarioRepo) ValidarPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// ─── Evento ───────────────────────────────────────────────────────────────────

type EventoRepo struct{ db *sqlx.DB }

func NewEventoRepo(db *sqlx.DB) *EventoRepo { return &EventoRepo{db: db} }

func (r *EventoRepo) Crear(ctx context.Context, req *models.CrearEventoRequest, adminID string) (*models.Evento, error) {
	var e models.Evento
	err := r.db.GetContext(ctx, &e, `
		INSERT INTO eventos (nombre, descripcion, creado_por)
		VALUES ($1, $2, $3)
		RETURNING id, nombre, descripcion, estado, creado_por, creado_en, terminado_en
	`, req.Nombre, req.Descripcion, adminID)
	return &e, err
}

func (r *EventoRepo) ObtenerActivo(ctx context.Context) (*models.Evento, error) {
	var e models.Evento
	err := r.db.GetContext(ctx, &e, `
		SELECT id, nombre, descripcion, estado, creado_por, creado_en, terminado_en
		FROM eventos WHERE estado = 'activo'
		ORDER BY creado_en DESC LIMIT 1
	`)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &e, err
}

func (r *EventoRepo) Listar(ctx context.Context) ([]models.Evento, error) {
	var lista []models.Evento
	err := r.db.SelectContext(ctx, &lista, `
		SELECT id, nombre, descripcion, estado, creado_por, creado_en, terminado_en
		FROM eventos ORDER BY creado_en DESC
	`)
	return lista, err
}

func (r *EventoRepo) Terminar(ctx context.Context, eventoID string) (*models.Evento, error) {
	ahora := time.Now()
	var e models.Evento
	err := r.db.GetContext(ctx, &e, `
		UPDATE eventos SET estado = 'terminado', terminado_en = $1
		WHERE id = $2 AND estado = 'activo'
		RETURNING id, nombre, descripcion, estado, creado_por, creado_en, terminado_en
	`, ahora, eventoID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("evento no encontrado o ya terminado")
	}
	return &e, err
}

// ─── Zona ─────────────────────────────────────────────────────────────────────

type ZonaRepo struct{ db *sqlx.DB }

func NewZonaRepo(db *sqlx.DB) *ZonaRepo { return &ZonaRepo{db: db} }

func (r *ZonaRepo) Crear(ctx context.Context, req *models.CrearZonaRequest, eventoID string) (*models.Zona, error) {
	var z models.Zona
	err := r.db.GetContext(ctx, &z, `
		INSERT INTO zonas (id, evento_id, nombre) VALUES ($1, $2, $3)
		RETURNING id, evento_id, nombre
	`, req.ID, eventoID, req.Nombre)
	return &z, err
}

func (r *ZonaRepo) Listar(ctx context.Context, eventoID string) ([]models.Zona, error) {
	var lista []models.Zona
	err := r.db.SelectContext(ctx, &lista, `
		SELECT id, evento_id, nombre FROM zonas WHERE evento_id = $1 ORDER BY nombre
	`, eventoID)
	return lista, err
}

func (r *ZonaRepo) Eliminar(ctx context.Context, zonaID, eventoID string) error {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM zonas WHERE id = $1 AND evento_id = $2
	`, zonaID, eventoID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("zona no encontrada")
	}
	return nil
}

// ─── Incidencia ───────────────────────────────────────────────────────────────

type IncidenciaRepo struct{ db *sqlx.DB }

func NewIncidenciaRepo(db *sqlx.DB) *IncidenciaRepo { return &IncidenciaRepo{db: db} }

func (r *IncidenciaRepo) Crear(ctx context.Context, req *models.CrearIncidenciaRequest, eventoID, adminID string) (*models.Incidencia, error) {
	var inc models.Incidencia
	err := r.db.GetContext(ctx, &inc, `
		WITH inserted AS (
			INSERT INTO incidencias (evento_id, zona_id, tipo, descripcion, creada_por, asignada_a)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING *
		)
		SELECT i.id, i.evento_id, i.zona_id, z.nombre as zona_nombre,
		       i.tipo, i.descripcion, i.estado, i.creada_por, i.asignada_a,
		       u.nombre as nombre_asignado,
		       i.creada_en, i.actualizada_en
		FROM inserted i
		LEFT JOIN zonas z ON z.id = i.zona_id AND z.evento_id = i.evento_id
		LEFT JOIN usuarios u ON u.id = i.asignada_a
	`, eventoID, req.ZonaID, req.Tipo, req.Descripcion, adminID, req.AsignadaA)
	return &inc, err
}

func (r *IncidenciaRepo) Listar(ctx context.Context, eventoID string) ([]models.Incidencia, error) {
	var lista []models.Incidencia
	err := r.db.SelectContext(ctx, &lista, `
		SELECT i.id, i.evento_id, i.zona_id, z.nombre as zona_nombre,
		       i.tipo, i.descripcion, i.estado, i.creada_por, i.asignada_a,
		       u.nombre as nombre_asignado,
		       i.creada_en, i.actualizada_en
		FROM incidencias i
		LEFT JOIN zonas z ON z.id = i.zona_id AND z.evento_id = i.evento_id
		LEFT JOIN usuarios u ON u.id = i.asignada_a
		WHERE i.evento_id = $1
		ORDER BY i.creada_en DESC
	`, eventoID)
	return lista, err
}

func (r *IncidenciaRepo) ObtenerPorID(ctx context.Context, id string) (*models.Incidencia, error) {
	var inc models.Incidencia
	err := r.db.GetContext(ctx, &inc, `
		SELECT i.id, i.evento_id, i.zona_id, z.nombre as zona_nombre,
		       i.tipo, i.descripcion, i.estado, i.creada_por, i.asignada_a,
		       u.nombre as nombre_asignado,
		       i.creada_en, i.actualizada_en
		FROM incidencias i
		LEFT JOIN zonas z ON z.id = i.zona_id AND z.evento_id = i.evento_id
		LEFT JOIN usuarios u ON u.id = i.asignada_a
		WHERE i.id = $1
	`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &inc, err
}

// Editar permite al trabajador cambiar estado, o al admin cambiar cualquier campo
func (r *IncidenciaRepo) Editar(ctx context.Context, id string, req *models.EditarIncidenciaRequest, usuarioID string) (*models.Incidencia, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Leer estado actual para historial
	var estadoActual string
	tx.QueryRowContext(ctx, `SELECT estado FROM incidencias WHERE id = $1`, id).Scan(&estadoActual)

	// Construir update dinámico
	if req.Estado != nil {
		_, err = tx.ExecContext(ctx, `UPDATE incidencias SET estado = $1 WHERE id = $2`, *req.Estado, id)
		if err != nil {
			return nil, err
		}
		// Historial
		tx.ExecContext(ctx, `
			INSERT INTO incidencias_historial (incidencia_id, estado_anterior, estado_nuevo, usuario_id)
			VALUES ($1, $2, $3, $4)
		`, id, estadoActual, *req.Estado, usuarioID)
	}
	if req.AsignadaA != nil {
		_, err = tx.ExecContext(ctx, `UPDATE incidencias SET asignada_a = $1 WHERE id = $2`, *req.AsignadaA, id)
		if err != nil {
			return nil, err
		}
	}

	tx.Commit()

	return r.ObtenerPorID(ctx, id)
}

// ─── Tarea ────────────────────────────────────────────────────────────────────

type TareaRepo struct{ db *sqlx.DB }

func NewTareaRepo(db *sqlx.DB) *TareaRepo { return &TareaRepo{db: db} }

func (r *TareaRepo) Crear(ctx context.Context, req *models.CrearTareaRequest, eventoID, adminID string) (*models.Tarea, error) {
	var zonaID *string
	if req.ZonaID != "" {
		zonaID = &req.ZonaID
	}
	var t models.Tarea
	err := r.db.GetContext(ctx, &t, `
		WITH inserted AS (
			INSERT INTO tareas (evento_id, zona_id, titulo, descripcion, prioridad, creada_por, asignada_a)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING *
		)
		SELECT t.id, t.evento_id, t.zona_id, z.nombre as zona_nombre,
		       t.titulo, t.descripcion, t.estado, t.prioridad,
		       t.creada_por, t.asignada_a, u.nombre as nombre_asignado,
		       t.completada_en, t.creada_en
		FROM inserted t
		LEFT JOIN zonas z ON z.id = t.zona_id AND z.evento_id = t.evento_id
		LEFT JOIN usuarios u ON u.id = t.asignada_a
	`, eventoID, zonaID, req.Titulo, req.Descripcion, req.Prioridad, adminID, req.AsignadaA)
	return &t, err
}

func (r *TareaRepo) Listar(ctx context.Context, eventoID string) ([]models.Tarea, error) {
	var lista []models.Tarea
	err := r.db.SelectContext(ctx, &lista, `
		SELECT t.id, t.evento_id, t.zona_id, z.nombre as zona_nombre,
		       t.titulo, t.descripcion, t.estado, t.prioridad,
		       t.creada_por, t.asignada_a, u.nombre as nombre_asignado,
		       t.completada_en, t.creada_en
		FROM tareas t
		LEFT JOIN zonas z ON z.id = t.zona_id AND z.evento_id = t.evento_id
		LEFT JOIN usuarios u ON u.id = t.asignada_a
		WHERE t.evento_id = $1
		ORDER BY
			CASE t.prioridad WHEN 'alta' THEN 1 WHEN 'media' THEN 2 ELSE 3 END,
			t.creada_en DESC
	`, eventoID)
	return lista, err
}

func (r *TareaRepo) ObtenerPorID(ctx context.Context, id string) (*models.Tarea, error) {
	var t models.Tarea
	err := r.db.GetContext(ctx, &t, `
		SELECT t.id, t.evento_id, t.zona_id, z.nombre as zona_nombre,
		       t.titulo, t.descripcion, t.estado, t.prioridad,
		       t.creada_por, t.asignada_a, u.nombre as nombre_asignado,
		       t.completada_en, t.creada_en
		FROM tareas t
		LEFT JOIN zonas z ON z.id = t.zona_id AND z.evento_id = t.evento_id
		LEFT JOIN usuarios u ON u.id = t.asignada_a
		WHERE t.id = $1
	`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &t, err
}

func (r *TareaRepo) Editar(ctx context.Context, id string, req *models.EditarTareaRequest) (*models.Tarea, error) {
	if req.Estado != nil {
		_, err := r.db.ExecContext(ctx, `UPDATE tareas SET estado = $1 WHERE id = $2`, *req.Estado, id)
		if err != nil {
			return nil, err
		}
	}
	if req.AsignadaA != nil {
		_, err := r.db.ExecContext(ctx, `UPDATE tareas SET asignada_a = $1 WHERE id = $2`, *req.AsignadaA, id)
		if err != nil {
			return nil, err
		}
	}
	return r.ObtenerPorID(ctx, id)
}

// ─── Mensaje ──────────────────────────────────────────────────────────────────

type MensajeRepo struct{ db *sqlx.DB }

func NewMensajeRepo(db *sqlx.DB) *MensajeRepo { return &MensajeRepo{db: db} }

func (r *MensajeRepo) Listar(ctx context.Context, eventoID string, limite int) ([]models.Mensaje, error) {
	var lista []models.Mensaje
	err := r.db.SelectContext(ctx, &lista, `
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
	// Invertir a orden cronológico
	for i, j := 0, len(lista)-1; i < j; i, j = i+1, j-1 {
		lista[i], lista[j] = lista[j], lista[i]
	}
	return lista, err
}

func (r *MensajeRepo) Crear(ctx context.Context, eventoID, usuarioID, contenido string) (*models.Mensaje, error) {
	var msg models.Mensaje
	err := r.db.GetContext(ctx, &msg, `
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
	`, eventoID, usuarioID, contenido)
	return &msg, err
}
