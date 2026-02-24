#!/bin/bash
# ============================================================
# Crear usuarios de prueba por rol
# Uso: bash scripts/seed_user.sh
# ============================================================

EVENTO_ID="a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"
PASSWORD="Test123!"

crear_usuario() {
    local EMAIL=$1
    local NOMBRE=$2
    local ROL=$3
    local HASH=$(htpasswd -bnBC 10 "" "$PASSWORD" | tr -d ':\n' | sed 's/\$2y\$/\$2a\$/')

    docker compose exec postgres psql -U eventpulse -d eventpulse_db -c "
        INSERT INTO usuarios (evento_id, nombre, email, password_hash, rol)
        VALUES ('$EVENTO_ID', '$NOMBRE', '$EMAIL', '$HASH', '$ROL')
        ON CONFLICT (email) DO UPDATE SET password_hash = EXCLUDED.password_hash;
    " > /dev/null

    echo "  ✅ $ROL — $NOMBRE ($EMAIL)"
}

echo "Creando usuarios de prueba..."
echo ""

crear_usuario "admin@eventpulse.com"      "Admin Sistema"     "admin"
crear_usuario "supervisor@eventpulse.com" "María Supervisor"  "supervisor"
crear_usuario "aseo1@eventpulse.com"      "Carlos Aseo"       "aseo"
crear_usuario "aseo2@eventpulse.com"      "Laura Aseo"        "aseo"
crear_usuario "guardia1@eventpulse.com"   "Pedro Guardia"     "guardia"
crear_usuario "guardia2@eventpulse.com"   "Ana Guardia"       "guardia"
crear_usuario "medico@eventpulse.com"     "Dr. Ramírez"       "medico"
crear_usuario "logistica@eventpulse.com"  "Sofia Logística"   "logistica"

echo ""
echo "════════════════════════════════════════"
echo "  Password para todos: $PASSWORD"
echo "  Evento ID: $EVENTO_ID"
echo "════════════════════════════════════════"
