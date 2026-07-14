package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"nihongo/internal/auth"
	"nihongo/internal/config"
	"nihongo/internal/user"
)

func NewRouter(pool *pgxpool.Pool, cfg config.Config) *gin.Engine {
	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	userRepo := user.NewRepository(pool)
	refreshRepo := user.NewRefreshRepository(pool)
	tokens := auth.NewTokenService(cfg.JWTSecret, auth.AccessTokenTTL)
	refresh := auth.NewRefreshService(refreshRepo, auth.RefreshTokenTTL)
	authHandler := auth.NewHandler(userRepo, tokens, refresh)

	authGroup := r.Group("/auth")
	{
		authGroup.POST("/register", authHandler.Register)
		authGroup.POST("/login", authHandler.Login)
		authGroup.POST("/refresh", authHandler.Refresh)
		authGroup.POST("/logout", authHandler.Logout)
	}

	protected := r.Group("/")
	protected.Use(auth.RequireAuth(tokens))
	{
		protected.GET("/me", authHandler.Me)
	}

	return r
}
