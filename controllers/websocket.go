package controllers

import (
	"go-api-boilerplate/services"

	"github.com/gin-gonic/gin"
)

type WebSocketController struct {
	wsService *services.WebSocketService
}

// NewWebSocketController creates a new WebSocket handler
func NewWebSocketController(wsService *services.WebSocketService) *WebSocketController {
	return &WebSocketController{
		wsService: wsService,
	}
}

// HandleWebSocket godoc
// @Summary WebSocket endpoint
// @Description Establish a WebSocket connection for real-time communication
// @Tags websocket
// @Security Bearer
// @Success 101 {string} string "Switching Protocols"
// @Failure 400 {object} utils.Response
// @Router /ws [get]
func (h *WebSocketController) HandleWebSocket(c *gin.Context) {
	// The WebSocket service handles the upgrade and connection
	h.wsService.HandleWebSocket(c)
}
