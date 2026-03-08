package http

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

type HttpResponse struct {
	Success bool   `json:"success"`
	Code    int    `json:"code"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}

func NewApp() *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      "seo-rank-guardian",
		ErrorHandler: defaultErrorHandler,
	})

	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "http://localhost:3000,http://localhost:5173",
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization",
		AllowCredentials: true,
	}))

	return app
}

func SuccessResponse(c *fiber.Ctx, code int, data any, message string) error {
	return c.Status(code).JSON(HttpResponse{
		Success: true,
		Code:    code,
		Data:    data,
		Message: message,
	})
}

func ErrorResponse(c *fiber.Ctx, code int, err string, message string) error {
	return c.Status(code).JSON(HttpResponse{
		Success: false,
		Code:    code,
		Error:   err,
		Message: message,
	})
}

func defaultErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}
	return ErrorResponse(c, code, err.Error(), "Something went wrong")
}
