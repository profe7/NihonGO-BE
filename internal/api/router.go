package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"nihongo/internal/auth"
	"nihongo/internal/config"
	"nihongo/internal/hiragana"
	"nihongo/internal/katakana"
	"nihongo/internal/user"
)

func NewRouter(pool *pgxpool.Pool, cfg config.Config) *gin.Engine {
	r := gin.Default()

	r.Use(corsMiddleware(cfg.CORSOrigin))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	userRepo := user.NewRepository(pool)
	refreshRepo := user.NewRefreshRepository(pool)
	tokens := auth.NewTokenService(cfg.JWTSecret, auth.AccessTokenTTL)
	refresh := auth.NewRefreshService(refreshRepo, auth.RefreshTokenTTL)
	authHandler := auth.NewHandler(userRepo, tokens, refresh)

	hiraganaRepo := hiragana.NewRepository(pool)
	hiraganaHandler := hiragana.NewHandler(hiraganaRepo)

	katakanaRepo := katakana.NewRepository(pool)
	katakanaHandler := katakana.NewHandler(katakanaRepo)

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

		protected.GET("/hiragana/quiz", hiraganaHandler.Quiz)
		protected.POST("/hiragana/quiz/answer", hiraganaHandler.Answer)
		protected.GET("/hiragana/stats", hiraganaHandler.Stats)

		protected.GET("/katakana/quiz", katakanaHandler.Quiz)
		protected.POST("/katakana/quiz/answer", katakanaHandler.Answer)
		protected.GET("/katakana/stats", katakanaHandler.Stats)
	}

	return r
}

func corsMiddleware(allowedOrigin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", allowedOrigin)
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
