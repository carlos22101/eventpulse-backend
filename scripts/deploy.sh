#!/bin/bash
# ============================================================
# EventPulse - Script de despliegue en AWS EC2 (Ubuntu 24.04)
# Ejecutar como: bash deploy.sh
#
# Requisitos previos:
#  - Instancia EC2 t3.small o superior
#  - Ubuntu 24.04 LTS
#  - Puerto 22 (SSH), 80 (HTTP), 443 (HTTPS) abiertos en Security Group
#  - Un dominio apuntando a la IP elástica de EC2
# ============================================================

set -e  # Salir ante cualquier error

DOMINIO="TU_DOMINIO.com"   # ← Cambiar
EMAIL="TU_EMAIL@gmail.com" # ← Cambiar (para Certbot)
REPO_URL="https://github.com/TU_USUARIO/eventpulse-backend.git" # ← Cambiar
APP_DIR="/opt/eventpulse"

echo "════════════════════════════════════════"
echo "  EventPulse Deploy Script"
echo "════════════════════════════════════════"

# ─── 1. Actualizar sistema ────────────────────────────────────────────────────
echo "▶ Actualizando sistema..."
sudo apt-get update -y
sudo apt-get upgrade -y

# ─── 2. Instalar Docker ───────────────────────────────────────────────────────
echo "▶ Instalando Docker..."
if ! command -v docker &> /dev/null; then
    curl -fsSL https://get.docker.com | sudo sh
    sudo usermod -aG docker $USER
    echo "✅ Docker instalado"
else
    echo "✅ Docker ya instalado"
fi

# Docker Compose plugin
sudo apt-get install -y docker-compose-plugin

# ─── 3. Instalar Nginx ────────────────────────────────────────────────────────
echo "▶ Instalando Nginx..."
sudo apt-get install -y nginx

# ─── 4. Instalar Certbot (Let's Encrypt) ─────────────────────────────────────
echo "▶ Instalando Certbot..."
sudo apt-get install -y certbot python3-certbot-nginx

# ─── 5. Clonar repositorio ────────────────────────────────────────────────────
echo "▶ Clonando repositorio..."
sudo mkdir -p $APP_DIR
sudo chown $USER:$USER $APP_DIR
if [ -d "$APP_DIR/.git" ]; then
    cd $APP_DIR && git pull
else
    git clone $REPO_URL $APP_DIR
fi
cd $APP_DIR

# ─── 6. Configurar variables de entorno ───────────────────────────────────────
echo "▶ Configurando .env..."
if [ ! -f "$APP_DIR/.env" ]; then
    cp .env.example .env
    echo ""
    echo "⚠️  IMPORTANTE: Edita el archivo .env antes de continuar:"
    echo "   nano $APP_DIR/.env"
    echo ""
    echo "Presiona ENTER cuando hayas configurado el .env..."
    read -r
fi

# ─── 7. Configurar Nginx (sin SSL primero) ────────────────────────────────────
echo "▶ Configurando Nginx..."
sudo cp scripts/nginx.conf /etc/nginx/sites-available/eventpulse

# Reemplazar dominio en la config
sudo sed -i "s/TU_DOMINIO.com/$DOMINIO/g" /etc/nginx/sites-available/eventpulse

# Crear directorio para Certbot
sudo mkdir -p /var/www/certbot

# Activar config temporal sin SSL para que Certbot pueda verificar el dominio
sudo tee /etc/nginx/sites-available/eventpulse-temp > /dev/null <<EOF
server {
    listen 80;
    server_name $DOMINIO;
    location /.well-known/acme-challenge/ {
        root /var/www/certbot;
    }
    location / {
        return 200 'OK';
    }
}
EOF

sudo ln -sf /etc/nginx/sites-available/eventpulse-temp /etc/nginx/sites-enabled/eventpulse
sudo rm -f /etc/nginx/sites-enabled/default
sudo nginx -t && sudo systemctl restart nginx

# ─── 8. Obtener certificado SSL ───────────────────────────────────────────────
echo "▶ Obteniendo certificado SSL con Let's Encrypt..."
sudo certbot certonly \
    --webroot \
    --webroot-path=/var/www/certbot \
    --email $EMAIL \
    --agree-tos \
    --no-eff-email \
    -d $DOMINIO

# Activar config completa con SSL
sudo ln -sf /etc/nginx/sites-available/eventpulse /etc/nginx/sites-enabled/eventpulse
sudo rm -f /etc/nginx/sites-available/eventpulse-temp
sudo nginx -t && sudo systemctl restart nginx

echo "✅ SSL configurado: https://$DOMINIO"

# ─── 9. Renovación automática de certificados ─────────────────────────────────
echo "▶ Configurando renovación automática de certificados..."
(sudo crontab -l 2>/dev/null; echo "0 12 * * * /usr/bin/certbot renew --quiet && systemctl reload nginx") | sudo crontab -

# ─── 10. Levantar aplicación con Docker Compose ───────────────────────────────
echo "▶ Construyendo y levantando contenedores..."
cd $APP_DIR
sudo docker compose build --no-cache
sudo docker compose up -d

echo ""
echo "▶ Esperando que los servicios estén listos..."
sleep 10

# Verificar que todo está corriendo
sudo docker compose ps

# ─── 11. Test de health check ─────────────────────────────────────────────────
echo ""
echo "▶ Verificando health check..."
if curl -sf "https://$DOMINIO/health" > /dev/null; then
    echo "✅ API funcionando en https://$DOMINIO"
else
    echo "⚠️  Health check falló. Revisar logs:"
    echo "   sudo docker compose logs app"
fi

# ─── 12. Configurar systemd para auto-start ───────────────────────────────────
echo "▶ Configurando inicio automático..."
sudo tee /etc/systemd/system/eventpulse.service > /dev/null <<EOF
[Unit]
Description=EventPulse Backend
After=docker.service
Requires=docker.service

[Service]
Type=oneshot
RemainAfterExit=yes
WorkingDirectory=$APP_DIR
ExecStart=/usr/bin/docker compose up -d
ExecStop=/usr/bin/docker compose down
TimeoutStartSec=0

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable eventpulse

echo ""
echo "════════════════════════════════════════"
echo "  ✅ Deploy completado exitosamente"
echo "════════════════════════════════════════"
echo ""
echo "  API REST:  https://$DOMINIO/api/v1"
echo "  WebSocket: wss://$DOMINIO/ws?token=..."
echo "  Health:    https://$DOMINIO/health"
echo ""
echo "Comandos útiles:"
echo "  Ver logs:       sudo docker compose -f $APP_DIR/docker-compose.yml logs -f app"
echo "  Reiniciar:      sudo docker compose -f $APP_DIR/docker-compose.yml restart app"
echo "  Ver DB:         sudo docker compose -f $APP_DIR/docker-compose.yml exec postgres psql -U eventpulse eventpulse_db"
