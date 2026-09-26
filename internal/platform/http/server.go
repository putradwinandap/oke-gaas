package http

import "github.com/gofiber/fiber/v3"

// New creates the HTTP delivery adapter for the Oke Gaas API.
func New() *fiber.App {
	app := fiber.New()

	app.Get("/health", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"status": "ok",
		})
	})

	return app
}
