package ws

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/eventpulse/backend/config"
	"github.com/eventpulse/backend/internal/models"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

// ─── Upgrader ─────────────────────────────────────────────────────────────────

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// En producción, validar el origen correctamente
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ─── Cliente ──────────────────────────────────────────────────────────────────

type Cliente struct {
	hub       *Hub
	conn      *websocket.Conn
	send      chan []byte
	UsuarioID string
	EventoID  string
}

func (c *Cliente) leerMensajes(maxMsgSize int64, pongWait time.Duration) {
	defer func() {
		c.hub.desregistrar <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMsgSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WS error cliente %s: %v", c.UsuarioID, err)
			}
			break
		}
		// El cliente no envía mensajes por WS (solo recibe).
		// Las acciones se hacen por REST → servidor publica en Redis.
	}
}

func (c *Cliente) escribirMensajes(pingPeriod time.Duration) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ─── Hub ──────────────────────────────────────────────────────────────────────
// El Hub mantiene todas las conexiones activas y distribuye mensajes.
// Redis Pub/Sub permite que múltiples instancias del servidor se comuniquen.

type Hub struct {
	// clientes agrupados por eventoID
	clientes     map[string]map[*Cliente]bool
	mu           sync.RWMutex
	registrar    chan *Cliente
	desregistrar chan *Cliente
	redis        *redis.Client
	cfg          *config.Config
}

func NewHub(redisClient *redis.Client, cfg *config.Config) *Hub {
	return &Hub{
		clientes:     make(map[string]map[*Cliente]bool),
		registrar:    make(chan *Cliente, 64),
		desregistrar: make(chan *Cliente, 64),
		redis:        redisClient,
		cfg:          cfg,
	}
}

func (h *Hub) Run(ctx context.Context) {
	// Suscribirse al canal de Redis para recibir eventos de otras instancias
	go h.suscribirRedis(ctx)

	for {
		select {
		case cliente := <-h.registrar:
			h.mu.Lock()
			if h.clientes[cliente.EventoID] == nil {
				h.clientes[cliente.EventoID] = make(map[*Cliente]bool)
			}
			h.clientes[cliente.EventoID][cliente] = true
			h.mu.Unlock()
			log.Printf("Cliente conectado: %s (evento: %s)", cliente.UsuarioID, cliente.EventoID)

		case cliente := <-h.desregistrar:
			h.mu.Lock()
			if conns, ok := h.clientes[cliente.EventoID]; ok {
				if _, ok := conns[cliente]; ok {
					delete(conns, cliente)
					close(cliente.send)
				}
			}
			h.mu.Unlock()
			log.Printf("Cliente desconectado: %s", cliente.UsuarioID)

		case <-ctx.Done():
			return
		}
	}
}

// PublicarEvento publica un evento en Redis para que llegue a TODAS las instancias.
func (h *Hub) PublicarEvento(ctx context.Context, eventoID string, evento models.EventoWS) error {
	data, err := json.Marshal(evento)
	if err != nil {
		return err
	}
	canal := "eventpulse:" + eventoID
	return h.redis.Publish(ctx, canal, data).Err()
}

// PublicarAUsuario envía un evento SOLO a un usuario específico (para rollbacks).
func (h *Hub) PublicarAUsuario(ctx context.Context, eventoID, usuarioID string, evento models.EventoWS) error {
	data, err := json.Marshal(evento)
	if err != nil {
		return err
	}
	canal := "eventpulse:usuario:" + usuarioID
	return h.redis.Publish(ctx, canal, data).Err()
}

// suscribirRedis escucha canales de Redis y distribuye a los clientes locales.
func (h *Hub) suscribirRedis(ctx context.Context) {
	// Patron: suscribirse a todos los canales de eventpulse
	pubsub := h.redis.PSubscribe(ctx, "eventpulse:*")
	defer pubsub.Close()

	for {
		select {
		case msg, ok := <-pubsub.Channel():
			if !ok {
				return
			}
			h.distribuirMensaje(msg.Channel, []byte(msg.Payload))

		case <-ctx.Done():
			return
		}
	}
}

func (h *Hub) distribuirMensaje(canal string, data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Canal de usuario específico: "eventpulse:usuario:{usuarioID}"
	// Canal de evento: "eventpulse:{eventoID}"
	var evento models.EventoWS
	if err := json.Unmarshal(data, &evento); err != nil {
		log.Printf("Error parseando evento WS: %v", err)
		return
	}

	if evento.EventoID == "" {
		return
	}

	for cliente := range h.clientes[evento.EventoID] {
		select {
		case cliente.send <- data:
		default:
			// Buffer lleno, cerrar conexión del cliente lento
			close(cliente.send)
			delete(h.clientes[evento.EventoID], cliente)
		}
	}
}

// ─── Handler de upgrade HTTP → WebSocket ─────────────────────────────────────

func (h *Hub) HandleConexion(w http.ResponseWriter, r *http.Request, usuarioID, eventoID string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrade WS: %v", err)
		return
	}

	cliente := &Cliente{
		hub:       h,
		conn:      conn,
		send:      make(chan []byte, 256),
		UsuarioID: usuarioID,
		EventoID:  eventoID,
	}

	h.registrar <- cliente

	pongWait := time.Duration(h.cfg.WS.PongWait) * time.Second
	pingPeriod := (pongWait * 9) / 10

	go cliente.escribirMensajes(pingPeriod)
	go cliente.leerMensajes(h.cfg.WS.MaxMessageSize, pongWait)
}
