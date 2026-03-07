package v1

import (
	_ "github.com/zeelrupapara/seo-rank-guardian/docs"
	swagger "github.com/swaggo/fiber-swagger"
)

func (h *HttpServer) RegisterV1() {
	h.App.Get("/swagger/*", swagger.WrapHandler)

	api := h.App.Group("/api")
	v1 := api.Group("/v1")

	v1.Get("/health", h.Health)

	auth := v1.Group("/auth")
	auth.Post("/register", h.Register)
	auth.Post("/login", h.Login)
	auth.Post("/refresh", h.RefreshToken)
	auth.Delete("/logout", h.Middleware.Protect(), h.Logout)
	auth.Get("/google", h.GoogleLogin)
	auth.Get("/google/callback", h.GoogleCallback)

	users := v1.Group("/users", h.Middleware.Protect())
	users.Get("/me", h.GetProfile)
	users.Put("/me", h.UpdateProfile)
}
