package main

import "github.com/zeelrupapara/seo-rank-guardian/cmd"

// @title SEO Rank Guardian API
// @version 1.0
// @description SEO Rank Guardian backend API with JWT auth, Google OAuth, and RBAC.
// @host localhost:8080
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	cmd.Execute()
}
