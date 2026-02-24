-- ============================================================
-- EventPulse - Schema PostgreSQL
-- Ejecutar en orden: psql -U eventpulse -d eventpulse_db -f 001_init.sql
-- ============================================================

-- Extensión para UUIDs
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ─── Eventos ──────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS eventos (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    nombre       VARCHAR(255) NOT NULL,
    descripcion  TEXT,
    fecha_inicio TIMESTAMPTZ NOT NULL,
    fecha_fin    TIMESTAMPTZ NOT NULL,
    activo       BOOLEAN DEFAULT true,
    creado_en    TIMESTAMPTZ DEFAULT NOW()
);

-- ─── Zonas ────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS zonas (
    id        UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    evento_id UUID NOT NULL REFERENCES eventos(id) ON DELETE CASCADE,
    nombre    VARCHAR(100) NOT NULL,
    pos_x     DECIMAL(5,2) DEFAULT 0,   -- Posición % en el mapa (0-100)
    pos_y     DECIMAL(5,2) DEFAULT 0,
    color     VARCHAR(7) DEFAULT '#3B82F6'
);

-- ─── Usuarios / Staff ─────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS usuarios (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    evento_id     UUID NOT NULL REFERENCES eventos(id) ON DELETE CASCADE,
    zona_id       UUID REFERENCES zonas(id) ON DELETE SET NULL,
    nombre        VARCHAR(255) NOT NULL,
    email         VARCHAR(255) UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    rol           VARCHAR(20) NOT NULL DEFAULT 'aseo'
                  CHECK (rol IN ('aseo', 'guardia', 'medico', 'logistica', 'supervisor', 'admin')),
    activo        BOOLEAN DEFAULT true,
    creado_en     TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_usuarios_email ON usuarios(email);
CREATE INDEX IF NOT EXISTS idx_usuarios_evento ON usuarios(evento_id);

-- ─── Incidencias ──────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS incidencias (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    evento_id       UUID NOT NULL REFERENCES eventos(id) ON DELETE CASCADE,
    zona_id         UUID NOT NULL REFERENCES zonas(id) ON DELETE CASCADE,
    tipo            VARCHAR(30) NOT NULL
                    CHECK (tipo IN ('derrame','seguridad','reabastecimiento','medico','otro')),
    descripcion     TEXT NOT NULL,
    estado          VARCHAR(20) NOT NULL DEFAULT 'pendiente'
                    CHECK (estado IN ('pendiente','en_atencion','resuelta')),
    reportada_por   UUID NOT NULL REFERENCES usuarios(id),
    atendida_por    UUID REFERENCES usuarios(id),
    creada_en       TIMESTAMPTZ DEFAULT NOW(),
    actualizada_en  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_incidencias_evento ON incidencias(evento_id);
CREATE INDEX IF NOT EXISTS idx_incidencias_estado ON incidencias(estado);
CREATE INDEX IF NOT EXISTS idx_incidencias_zona ON incidencias(zona_id);

-- Trigger para actualizar actualizada_en automáticamente
CREATE OR REPLACE FUNCTION update_actualizada_en()
RETURNS TRIGGER AS $$
BEGIN
    NEW.actualizada_en = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_incidencias_actualizada_en
    BEFORE UPDATE ON incidencias
    FOR EACH ROW EXECUTE FUNCTION update_actualizada_en();

-- Historial de incidencias (auditoría y rollback)
CREATE TABLE IF NOT EXISTS incidencias_historial (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    incidencia_id   UUID NOT NULL REFERENCES incidencias(id) ON DELETE CASCADE,
    estado_anterior VARCHAR(20),
    estado_nuevo    VARCHAR(20),
    usuario_id      UUID REFERENCES usuarios(id),
    cambiado_en     TIMESTAMPTZ DEFAULT NOW(),
    notas           TEXT
);

-- ─── Tareas ───────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS tareas (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    evento_id     UUID NOT NULL REFERENCES eventos(id) ON DELETE CASCADE,
    zona_id       UUID REFERENCES zonas(id) ON DELETE SET NULL,
    titulo        VARCHAR(255) NOT NULL,
    descripcion   TEXT,
    estado        VARCHAR(20) NOT NULL DEFAULT 'pendiente'
                  CHECK (estado IN ('pendiente','completada')),
    prioridad     VARCHAR(10) NOT NULL DEFAULT 'media'
                  CHECK (prioridad IN ('alta','media','baja')),
    asignada_a    UUID REFERENCES usuarios(id),
    completada_en TIMESTAMPTZ,
    creada_en     TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tareas_evento ON tareas(evento_id);
CREATE INDEX IF NOT EXISTS idx_tareas_estado ON tareas(estado);

-- ─── Chat / Mensajes ──────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS mensajes (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    evento_id   UUID NOT NULL REFERENCES eventos(id) ON DELETE CASCADE,
    usuario_id  UUID NOT NULL REFERENCES usuarios(id),
    contenido   TEXT NOT NULL CHECK (LENGTH(contenido) <= 500),
    enviado_en  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_mensajes_evento ON mensajes(evento_id);
CREATE INDEX IF NOT EXISTS idx_mensajes_enviado ON mensajes(enviado_en DESC);

-- ─── Datos de ejemplo para desarrollo ────────────────────────────────────────
INSERT INTO eventos (id, nombre, descripcion, fecha_inicio, fecha_fin)
VALUES (
    'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
    'Festival de Música 2024',
    'Festival anual en el estadio central',
    NOW(),
    NOW() + INTERVAL '3 days'
) ON CONFLICT DO NOTHING;

INSERT INTO zonas (evento_id, nombre, pos_x, pos_y, color) VALUES
    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'Entrada Principal', 50, 5, '#3B82F6'),
    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'Escenario Central', 50, 40, '#8B5CF6'),
    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'Pasillo 1', 20, 50, '#10B981'),
    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'Pasillo 2', 80, 50, '#10B981'),
    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'Zona VIP', 50, 70, '#F59E0B'),
    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'Baños Norte', 20, 90, '#6B7280'),
    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'Baños Sur', 80, 90, '#6B7280')
ON CONFLICT DO NOTHING;
