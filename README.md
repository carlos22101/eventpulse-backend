# EventPulse Backend

API REST + WebSocket para coordinación de staff en eventos masivos.  
Stack: **Go + Gin + PostgreSQL + Redis + Docker + Nginx (HTTPS/WSS)**

---

## Requisitos locales

- Go 1.22+
- Docker + Docker Compose
- Make (opcional)

---

## Desarrollo local

```bash
# 1. Clonar y entrar al proyecto
git clone https://github.com/TU_USUARIO/eventpulse-backend.git
cd eventpulse-backend

# 2. Configurar variables de entorno
cp .env.example .env
# Editar .env con tus valores

# 3. Levantar PostgreSQL y Redis
docker compose up postgres redis -d

# 4. Ejecutar migraciones (solo primera vez)
docker compose exec postgres psql -U eventpulse -d eventpulse_db -f /docker-entrypoint-initdb.d/001_init.sql

# 5. Crear usuario de prueba
bash scripts/seed_user.sh

# 6. Correr la app
go run ./cmd/server

# O todo junto con Docker:
docker compose up --build
```

La API estará en: `http://localhost:8080`

---

## Deploy en AWS EC2

```bash
# En tu instancia EC2 (Ubuntu 24.04):
git clone https://github.com/TU_USUARIO/eventpulse-backend.git
cd eventpulse-backend

# Editar deploy.sh con tu dominio y email
nano scripts/deploy.sh

# Ejecutar
bash scripts/deploy.sh
```

**Requisitos de la instancia EC2:**
- Tipo: t3.small mínimo (t3.medium recomendado)
- OS: Ubuntu 24.04 LTS
- Security Group: puertos 22, 80, 443 abiertos
- IP elástica asignada
- Dominio apuntando a la IP elástica (registro A en tu DNS)

---

## Endpoints de la API

### Auth

| Método | Ruta | Auth | Descripción |
|--------|------|------|-------------|
| POST | `/api/v1/auth/login` | ❌ | Login, retorna JWT |
| GET | `/api/v1/auth/me` | ✅ | Perfil del usuario actual |

**Login:**
```json
POST /api/v1/auth/login
{
  "email": "admin@eventpulse.com",
  "password": "Admin123!"
}
```
```json
// Respuesta
{
  "token": "eyJhbGci...",
  "usuario": {
    "id": "uuid",
    "nombre": "Admin EventPulse",
    "email": "admin@eventpulse.com",
    "rol": "admin",
    "evento_id": "uuid"
  }
}
```

### Incidencias

| Método | Ruta | Auth | Descripción |
|--------|------|------|-------------|
| GET | `/api/v1/incidencias` | ✅ | Listar todas del evento |
| POST | `/api/v1/incidencias` | ✅ | Reportar nueva incidencia |
| PATCH | `/api/v1/incidencias/:id/atender` | ✅ | Tomar incidencia (con lock) |
| PATCH | `/api/v1/incidencias/:id/resolver` | ✅ | Marcar como resuelta |

**Reportar incidencia:**
```json
POST /api/v1/incidencias
Authorization: Bearer <token>
{
  "zona_id": "uuid-de-la-zona",
  "tipo": "derrame",
  "descripcion": "Derrame de líquido en pasillo 4"
}
// tipos: derrame | seguridad | reabastecimiento | medico | otro
```

**Atender (flujo optimista):**
```
PATCH /api/v1/incidencias/{id}/atender
Authorization: Bearer <token>

→ 200 OK: Incidencia tomada exitosamente
→ 409 Conflict: Otra persona la tomó primero
  + Se envía evento WSIncidenciaConflicto al usuario por WebSocket (rollback)
```

### Tareas

| Método | Ruta | Auth | Descripción |
|--------|------|------|-------------|
| GET | `/api/v1/tareas` | ✅ | Listar tareas del evento |
| PATCH | `/api/v1/tareas/:id/completar` | ✅ | Marcar tarea como completada |

### Chat

| Método | Ruta | Auth | Descripción |
|--------|------|------|-------------|
| GET | `/api/v1/chat/historial` | ✅ | Últimos 50 mensajes |
| POST | `/api/v1/chat/mensaje` | ✅ | Enviar mensaje |

---

## WebSocket

**Conexión:**
```
wss://TU_DOMINIO.com/ws?token=<JWT>
// Local: ws://localhost:8080/ws?token=<JWT>
```

El token se pasa como query param porque los clientes WebSocket nativos de Android no soportan headers custom en el handshake inicial.

**Eventos que recibe el cliente (JSON):**

```json
// Nueva incidencia reportada
{
  "tipo": "incidencia_nueva",
  "evento_id": "uuid",
  "payload": { /* objeto Incidencia completo */ }
}

// Incidencia actualizada (atendida o resuelta)
{
  "tipo": "incidencia_actualizada",
  "evento_id": "uuid",
  "payload": { /* objeto Incidencia */ }
}

// Rollback: conflicto de concurrencia (solo al usuario afectado)
{
  "tipo": "incidencia_conflicto",
  "evento_id": "uuid",
  "payload": {
    "incidencia_id": "uuid",
    "mensaje": "La incidencia ya fue tomada por Juan García",
    "atendida_por": "Juan García"
  }
}

// Tarea actualizada
{
  "tipo": "tarea_actualizada",
  "evento_id": "uuid",
  "payload": { /* objeto Tarea */ }
}

// Mensaje de chat
{
  "tipo": "mensaje_nuevo",
  "evento_id": "uuid",
  "payload": { /* objeto Mensaje */ }
}
```

---

## Estructura del proyecto

```
eventpulse-backend/
├── cmd/server/main.go          ← Punto de entrada
├── config/config.go            ← Variables de entorno
├── internal/
│   ├── auth/jwt.go             ← Generación y validación JWT
│   ├── db/db.go                ← Conexiones PostgreSQL y Redis
│   ├── handlers/handlers.go    ← Controladores HTTP
│   ├── middleware/auth.go      ← Middleware JWT
│   ├── models/models.go        ← Modelos de dominio y DTOs
│   ├── repository/repository.go← Acceso a datos
│   └── ws/hub.go               ← Hub WebSocket + Redis Pub/Sub
├── migrations/001_init.sql     ← Schema de la base de datos
├── scripts/
│   ├── nginx.conf              ← Config Nginx (HTTPS + WSS)
│   ├── deploy.sh               ← Script de deploy en EC2
│   └── seed_user.sh            ← Crear usuario de prueba
├── Dockerfile                  ← Build multistage
├── docker-compose.yml          ← Orquestación local
└── .env.example                ← Variables de entorno ejemplo
```

---

## Variables de entorno importantes

| Variable | Descripción | Ejemplo |
|----------|-------------|---------|
| `JWT_SECRET` | Clave secreta JWT (mín. 32 chars) | `cambiarEnProd_abc123xyz...` |
| `DB_PASSWORD` | Password de PostgreSQL | `superSecure!` |
| `DB_SSLMODE` | SSL en la DB | `disable` (local) / `require` (RDS) |
| `ENV` | Entorno actual | `development` / `production` |

---

## Comandos útiles post-deploy

```bash
# Ver logs en tiempo real
sudo docker compose logs -f app

# Ver conexiones WebSocket activas (Redis)
sudo docker compose exec redis redis-cli pubsub channels

# Conectar a PostgreSQL
sudo docker compose exec postgres psql -U eventpulse eventpulse_db

# Reiniciar solo la app (sin reiniciar DB)
sudo docker compose restart app

# Ver estado de los contenedores
sudo docker compose ps

# Renovar SSL manualmente
sudo certbot renew
```
