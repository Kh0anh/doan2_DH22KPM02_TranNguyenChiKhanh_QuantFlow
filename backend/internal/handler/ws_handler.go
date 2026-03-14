package handler

// ws_handler.go — WebSocket upgrade handler (Task 2.8.1).
//
// WSHandler upgrades an HTTP GET /api/v1/ws request to a WebSocket connection
// after authenticating the caller via JWT.
//
// ─── Authentication Flow ─────────────────────────────────────────────────────
//
// The handler checks for a JWT token using two methods, in order:
//  1. Query parameter: GET /v1/ws?token=<JWT>
//  2. HttpOnly Cookie: automatically sent by the browser during WS handshake.
//
// If no valid token is found the handler writes a Close frame with code 4001
// and an AUTH_FAILED payload before returning — the connection is NOT upgraded
// (websocket.md §1.2).
//
// Note: because auth is validated by this handler itself, the route MUST be
// registered OUTSIDE the JWTAuth middleware group in router.go.
//
// ─── Goroutine Model ─────────────────────────────────────────────────────────
//
// After a successful upgrade and authentication:
//  1. A *websocket.Client is constructed with the verified userID and conn.
//  2. The client is registered in WSManager.
//  3. writePump is launched in a new goroutine.
//  4. readPump is called on the current goroutine (blocking until disconnect).
//  5. When readPump returns, UnregisterClient has already been called (deferred
//     inside readPump), which closes the send channel and causes writePump to
//     exit cleanly.
//
// Task 2.8.1 — WebSocket Server Connection Manager (JWT Auth + Heartbeat 30s).
// WBS: P2-Backend · 15/03/2026.
// SRS: NFR-SEC-04, FR-MON.

import (
	"log/slog"
	"net/http"

	"github.com/gorilla/websocket"
	appws "github.com/kh0anh/quantflow/internal/websocket"
	pkgcrypto "github.com/kh0anh/quantflow/pkg/crypto"
)

// tokenCookieName is the HttpOnly cookie name set by auth_handler on login.
// Duplicated here (rather than importing the middleware package) to keep the
// handler package free of a middleware import cycle.
const wsCookieName = "token"

// upgrader configures the gorilla/websocket HTTP-to-WS upgrade.
// CheckOrigin always returns true because origin validation is handled at the
// Nginx reverse-proxy layer (SameSite=Lax, CORS headers — SRS NFR-SEC-04).
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// WSHandler handles the HTTP → WebSocket upgrade for the /ws endpoint.
type WSHandler struct {
	manager   *appws.WSManager
	jwtSecret string
	logger    *slog.Logger
}

// NewWSHandler constructs a WSHandler.
//
//   - manager:   the singleton WSManager that tracks all active connections.
//   - jwtSecret: the HS256 signing secret used to validate JWT tokens.
//   - logger:    slog.Logger; slog.Default() is used when nil.
func NewWSHandler(manager *appws.WSManager, jwtSecret string, logger *slog.Logger) *WSHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &WSHandler{
		manager:   manager,
		jwtSecret: jwtSecret,
		logger:    logger,
	}
}

// ServeWS handles GET /api/v1/ws.
//
// Flow:
//  1. Extract JWT token from query param "token" → fallback to cookie "token".
//  2. Validate the JWT.  On failure: upgrade, send Close 4001 AUTH_FAILED, return.
//  3. Upgrade the HTTP connection to WebSocket.
//  4. Construct Client, register in WSManager.
//  5. Launch writePump in a goroutine; run readPump on this goroutine.
func (h *WSHandler) ServeWS(w http.ResponseWriter, r *http.Request) {
	// ── Step 1: Extract token ─────────────────────────────────────────────
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		cookie, err := r.Cookie(wsCookieName)
		if err == nil {
			tokenStr = cookie.Value
		}
	}

	// ── Step 2: Validate JWT ──────────────────────────────────────────────
	claims, authErr := pkgcrypto.ParseToken(tokenStr, h.jwtSecret)

	// ── Step 3: Upgrade the connection regardless of auth result ──────────
	// We must upgrade before we can send a WebSocket Close frame.
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		// Upgrade itself failed (e.g. not a WS request) — HTTP error already
		// written by gorilla; just log and return.
		h.logger.Debug("ws: upgrade failed", slog.Any("error", err))
		return
	}

	// ── Step 4: Enforce auth — send Close 4001 if invalid ─────────────────
	if authErr != nil || claims == nil {
		h.logger.Debug("ws: auth failed — closing with 4001",
			slog.String("remote", r.RemoteAddr),
			slog.Any("error", authErr),
		)
		// Write Close frame with application-defined code 4001 (websocket.md §1.2).
		closeMsg := websocket.FormatCloseMessage(4001, "AUTH_FAILED")
		_ = conn.WriteMessage(websocket.CloseMessage, closeMsg)
		conn.Close()
		return
	}

	// ── Step 5: Register client and start pump goroutines ─────────────────
	clientLogger := h.logger.With(
		slog.String("user_id", claims.UserID),
		slog.String("remote", r.RemoteAddr),
	)
	client := appws.NewClient(claims.UserID, conn, clientLogger)
	h.manager.RegisterClient(client)

	clientLogger.Debug("ws: connection established")

	// writePump runs concurrently; readPump blocks until the connection closes.
	go client.WritePump(h.manager)
	client.ReadPump(h.manager)
}
