package webhook

import (
	"errors"
	"fmt"
	"log"
	"net"

	"github.com/gofiber/fiber/v2"
)

// Server wraps a GoFiber HTTP server for handling incoming webhook requests.
type Server struct {
	app     *fiber.App
	manager *Manager
}

// NewServer creates a new webhook HTTP server with a route for ALL /webhooks/:uuid.
func NewServer(manager *Manager) *Server {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		BodyLimit:             256 * 1024, // 256KB max body for webhook payloads
	})

	s := &Server{
		app:     app,
		manager: manager,
	}

	app.All("/webhooks/:uuid", s.handleWebhook)

	return s
}

// Listen starts the HTTP server on the given address.
// It pre-validates that the address can be bound before starting the server.
func (s *Server) Listen(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("webhook: bind %s: %w", addr, err)
	}
	log.Printf("webhook: server listening on %s", addr)
	return s.app.Listener(ln)
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown() error {
	return s.app.Shutdown()
}

func (s *Server) handleWebhook(c *fiber.Ctx) error {
	id := c.Params("uuid")

	req := IncomingRequest{
		Method:  c.Method(),
		Headers: extractHeaders(c),
		Query:   c.Queries(),
		Body:    string(c.Body()),
	}

	if err := s.manager.HandleIncoming(id, req); err != nil {
		if errors.Is(err, ErrWebhookNotFound) || errors.Is(err, ErrWebhookInactive) {
			return c.SendStatus(fiber.StatusNotFound)
		}
		if errors.Is(err, ErrWebhookBusy) {
			return c.SendStatus(fiber.StatusTooManyRequests)
		}
		log.Printf("webhook: handle incoming %s: %v", id, err)
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.SendStatus(fiber.StatusOK)
}

func extractHeaders(c *fiber.Ctx) map[string]string {
	headers := make(map[string]string)
	c.Request().Header.VisitAll(func(key, value []byte) {
		k := string(key)
		if existing, ok := headers[k]; ok {
			headers[k] = existing + ", " + string(value)
		} else {
			headers[k] = string(value)
		}
	})
	return headers
}
