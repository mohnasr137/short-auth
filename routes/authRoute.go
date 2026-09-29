package routes

import (
	"mohnasr137/short-auth/controllers"
	"mohnasr137/short-auth/internal"
	"mohnasr137/short-auth/middlewares"

	"github.com/gin-gonic/gin"
)

func AuthRoutes(server *gin.Engine, app *internal.Application) {
	auth := server.Group("/api/auth")
	{
		// Login endpoint: rate-limited to 15 req/min per IP to protect against brute force
		auth.POST(
			"/login",
			middlewares.RateLimit(15, 5),
			controllers.LoginHandler(app),
		)

		// Refresh token endpoint
		auth.POST(
			"/refresh",
			middlewares.RateLimit(30, 10),
			controllers.RefreshTokenHandler(app),
		)

		// Logout endpoint (invalidates refresh token session in Keycloak)
		auth.POST(
			"/logout",
			controllers.LogoutHandler(app),
		)

		// Google Login redirect URL endpoint
		auth.GET(
			"/google",
			controllers.GoogleLoginHandler(app),
		)

		// Registration endpoint: rate-limited to 10 req/min per IP to prevent spam registrations
		auth.POST(
			"/register",
			middlewares.RateLimit(10, 5),
			controllers.RegisterHandler(app),
		)

		// Password change endpoint: requires authenticated token and rate-limited
		auth.PUT(
			"/change-password",
			middlewares.AuthMiddleware(app),
			middlewares.RateLimit(10, 5),
			controllers.ChangePasswordHandler(app),
		)

		// OAuth token exchange callback (supports browser redirect GET and SPA JSON POST)
		auth.GET(
			"/callback",
			middlewares.RateLimit(20, 10),
			controllers.OAuthCallbackHandler(app),
		)
		auth.POST(
			"/callback",
			middlewares.RateLimit(20, 10),
			controllers.OAuthCallbackHandler(app),
		)

		// Current user profile: protected by OIDC access token verification
		auth.GET(
			"/me",
			middlewares.AuthMiddleware(app),
			controllers.MeHandler(app),
		)

		// Token verification and introspection for downstream services / gateways
		auth.GET(
			"/verify",
			middlewares.AuthMiddleware(app),
			controllers.VerifyTokenHandler(app),
		)
		auth.POST(
			"/verify",
			middlewares.AuthMiddleware(app),
			controllers.VerifyTokenHandler(app),
		)

		// Forgot password: strictly rate-limited (5 req/min per IP) to prevent email spam & enumeration
		auth.POST(
			"/forgot-password",
			middlewares.RateLimit(5, 3),
			controllers.ForgotPasswordHandler(app),
		)
	}
}
