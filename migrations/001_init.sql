-- ============================================================
-- EventPulse v2 - Schema PostgreSQL
-- ============================================================

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ─── Admin único (creado directo, no por ruta) ────────────────────────────────
-- El admin existe desde el inicio y no pertenece a un evento específico.
-- Los trabajadores sí se vinculan al evento activo cuando el admin los crea.

CREATE TABLE IF NOT EXISTS usuarios (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    nombre_usuario VARCHAR(50) UNIQUE NOT NULL,   -- username para login
    nombre         VARCHAR(255) NOT NULL,          -- nombre visible en chat
    password_hash  TEXT NOT NULL,
    rol            VARCHAR(20) NOT NULL DEFAULT 'aseo'
                   CHECK (rol IN ('admin','aseo','guardia','medico','logistica','supervisor')),
    evento_id      UUID,                           -- NULL si es admin o no vinculado
    activo         BOOLEAN DEFAULT true,
    creado_en      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_usuarios_nombre_usuario ON usuarios(nombre_usuario);

-- ─── Eventos ──────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS eventos (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    nombre       VARCHAR(255) NOT NULL,
    descripcion  TEXT,
    estado       VARCHAR(20) NOT NULL DEFAULT 'activo'
                 CHECK (estado IN ('activo','terminado')),
    creado_por   UUID NOT NULL REFERENCES usuarios(id),
    creado_en    TIMESTAMPTZ DEFAULT NOW(),
    terminado_en TIMESTAMPTZ
);

-- FK de usuarios → eventos (se agrega después para evitar referencia circular)
ALTER TABLE usuarios ADD CONSTRAINT fk_usuarios_evento
    FOREIGN KEY (evento_id) REFERENCES eventos(id) ON DELETE SET NULL;

-- ─── Zonas ────────────────────────────────────────────────────────────────────
-- ID manual definido por el admin ej: "bano-norte", "pasillo-4", "entrada"
CREATE TABLE IF NOT EXISTS zonas (
    id        VARCHAR(50) NOT NULL,              -- ID legible, manual
    evento_id UUID NOT NULL REFERENCES eventos(id) ON DELETE CASCADE,
    nombre    VARCHAR(100) NOT NULL,
    PRIMARY KEY (id, evento_id)                  -- único por evento
);

-- ─── Incidencias ──────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS incidencias (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    evento_id      UUID NOT NULL REFERENCES eventos(id) ON DELETE CASCADE,
    zona_id        VARCHAR(50) NOT NULL,
    tipo           VARCHAR(30) NOT NULL
                   CHECK (tipo IN ('derrame','seguridad','reabastecimiento','medico','otro')),
    descripcion    TEXT NOT NULL,
    estado         VARCHAR(20) NOT NULL DEFAULT 'pendiente'
                   CHECK (estado IN ('pendiente','en_atencion','resuelta')),
    creada_por     UUID NOT NULL REFERENCES usuarios(id),  -- siempre el admin
    asignada_a     UUID REFERENCES usuarios(id),           -- trabajador asignado
    creada_en      TIMESTAMPTZ DEFAULT NOW(),
    actualizada_en TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_incidencias_evento  ON incidencias(evento_id);
CREATE INDEX IF NOT EXISTS idx_incidencias_estado  ON incidencias(estado);

-- Trigger actualizada_en
CREATE OR REPLACE FUNCTION update_actualizada_en()
RETURNS TRIGGER AS $$
BEGIN NEW.actualizada_en = NOW(); RETURN NEW; END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_incidencias_upd
    BEFORE UPDATE ON incidencias
    FOR EACH ROW EXECUTE FUNCTION update_actualizada_en();

-- Historial
CREATE TABLE IF NOT EXISTS incidencias_historial (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    incidencia_id   UUID NOT NULL REFERENCES incidencias(id) ON DELETE CASCADE,
    estado_anterior VARCHAR(20),
    estado_nuevo    VARCHAR(20),
    usuario_id      UUID REFERENCES usuarios(id),
    cambiado_en     TIMESTAMPTZ DEFAULT NOW()
);

-- ─── Tareas ───────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS tareas (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    evento_id     UUID NOT NULL REFERENCES eventos(id) ON DELETE CASCADE,
    zona_id       VARCHAR(50),
    titulo        VARCHAR(255) NOT NULL,
    descripcion   TEXT,
    estado        VARCHAR(20) NOT NULL DEFAULT 'pendiente'
                  CHECK (estado IN ('pendiente','en_progreso','completada')),
    prioridad     VARCHAR(10) NOT NULL DEFAULT 'media'
                  CHECK (prioridad IN ('alta','media','baja')),
    creada_por    UUID NOT NULL REFERENCES usuarios(id),
    asignada_a    UUID REFERENCES usuarios(id),
    completada_en TIMESTAMPTZ,
    creada_en     TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tareas_evento ON tareas(evento_id);
CREATE INDEX IF NOT EXISTS idx_tareas_estado ON tareas(estado);

CREATE OR REPLACE FUNCTION update_tareas_actualizada_en()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.estado = 'completada' AND OLD.estado != 'completada' THEN
        NEW.completada_en = NOW();
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_tareas_completada
    BEFORE UPDATE ON tareas
    FOR EACH ROW EXECUTE FUNCTION update_tareas_actualizada_en();

-- ─── Mensajes Chat ────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS mensajes (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    evento_id  UUID NOT NULL REFERENCES eventos(id) ON DELETE CASCADE,
    usuario_id UUID NOT NULL REFERENCES usuarios(id),
    contenido  TEXT NOT NULL CHECK (LENGTH(contenido) <= 500),
    enviado_en TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_mensajes_evento  ON mensajes(evento_id);
CREATE INDEX IF NOT EXISTS idx_mensajes_enviado ON mensajes(enviado_en DESC);


INSERT INTO usuarios (nombre_usuario, nombre, password_hash, rol, evento_id)
VALUES ('admin', 'Administrador', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LPVKoYDhHS6', 'admin', NULL)
ON CONFLICT (nombre_usuario) DO NOTHING;
