# ─── Stage 1: Build ───────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

# Instalar dependencias del sistema
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Copiar módulos primero (mejor cache de Docker)
COPY go.mod go.sum ./
RUN go mod download

# Copiar código fuente
COPY . .

# Compilar binario estático
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /app/eventpulse \
    ./cmd/server

# ─── Stage 2: Runtime ─────────────────────────────────────────────────────────
# Imagen mínima: solo el binario y los certificados SSL
FROM scratch

# Certificados para HTTPS outbound (llamadas a servicios externos)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Binario compilado
COPY --from=builder /app/eventpulse /eventpulse

# Puerto de la app
EXPOSE 8080

ENTRYPOINT ["/eventpulse"]
