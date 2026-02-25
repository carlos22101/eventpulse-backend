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

var upgrader = websocket.Upgrader{
	ReadBufferSize:  2048,
	WriteBufferSize: 2048,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// ─── Cliente ──────────────────────────────────────────────────────────────────

type Cliente struct {
	hub       *Hub
	conn      *websocket.Conn
	send      chan []byte
	UsuarioID string
	EventoID  string
}

func (c *Cliente) leer(maxSize int64, pongWait time.Duration) {
	defer func() {
		c.hub.desregistrar <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		// El cliente solo recibe. No procesa mensajes entrantes por WS.
		// Todo se envía por REST y se distribuye por WS.
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WS cliente %s cerró inesperadamente: %v", c.UsuarioID, err)
			}
			break
		}
	}
}

func (c *Cliente) escribir(pingPeriod time.Duration) {
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

type Hub struct {
	clientes     map[string]map[*Cliente]bool // eventoID → clientes
	mu           sync.RWMutex
	registrar    chan *Cliente
	desregistrar chan *Cliente
	redis        *redis.Client
	cfg          *config.Config
}

func NewHub(r *redis.Client, cfg *config.Config) *Hub {
	return &Hub{
		clientes:     make(map[string]map[*Cliente]bool),
		registrar:    make(chan *Cliente, 64),
		desregistrar: make(chan *Cliente, 64),
		redis:        r,
		cfg:          cfg,
	}
}

func (h *Hub) Run(ctx context.Context) {
	go h.escucharRedis(ctx)
	for {
		select {
		case c := <-h.registrar:
			h.mu.Lock()
			if h.clientes[c.EventoID] == nil {
				h.clientes[c.EventoID] = make(map[*Cliente]bool)
			}
			h.clientes[c.EventoID][c] = true
			h.mu.Unlock()
			log.Printf("✅ WS conectado: %s (evento: %s)", c.UsuarioID, c.EventoID)
			log.Println("🔥 HUB RUNNING")

		case c := <-h.desregistrar:
			h.mu.Lock()
			if conns, ok := h.clientes[c.EventoID]; ok {
				if _, ok := conns[c]; ok {
					delete(conns, c)
					close(c.send)
				}
			}
			h.mu.Unlock()
			log.Printf("❌ WS desconectado: %s", c.UsuarioID)

		case <-ctx.Done():
			return
		}
	}
}

// Publicar envía un evento a TODOS los conectados al evento (incluye admin y trabajadores)
func (h *Hub) Publicar(ctx context.Context, eventoID string, evento models.EventoWS) error {
	data, err := json.Marshal(evento)
	if err != nil {
		return err
	}
	return h.redis.Publish(ctx, "ep:evento:"+eventoID, data).Err()
}

func (h *Hub) escucharRedis(ctx context.Context) {
	pubsub := h.redis.PSubscribe(ctx, "ep:evento:*")
	defer pubsub.Close()

	log.Println("👂 Escuchando Redis...")

	for {
		select {
		case msg, ok := <-pubsub.Channel():
			if !ok {
				return
			}

			log.Println("📩 Mensaje recibido de Redis:", msg.Channel)

			var evento models.EventoWS
			if err := json.Unmarshal([]byte(msg.Payload), &evento); err != nil {
				log.Println("❌ Error deserializando:", err)
				continue
			}

			h.distribuir(evento.EventoID, []byte(msg.Payload))

		case <-ctx.Done():
			return
		}
	}
}

func (h *Hub) distribuir(eventoID string, data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clientes[eventoID] {
		select {
		case c.send <- data:
		default:
			close(c.send)
			delete(h.clientes[eventoID], c)
		}
	}
}

// HandleConexion hace el upgrade HTTP→WS y registra el cliente
func (h *Hub) HandleConexion(w http.ResponseWriter, r *http.Request, usuarioID, eventoID string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrade WS: %v", err)
		return
	}
	c := &Cliente{
		hub:       h,
		conn:      conn,
		send:      make(chan []byte, 512),
		UsuarioID: usuarioID,
		EventoID:  eventoID,
	}
	h.registrar <- c

	pongWait := time.Duration(h.cfg.WS.PongWait) * time.Second
	pingPeriod := (pongWait * 9) / 10

	go c.escribir(pingPeriod)
	go c.leer(h.cfg.WS.MaxMessageSize, pongWait)
}
