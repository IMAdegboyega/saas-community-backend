package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/rs/cors"

	"github.com/tommygebru/kiekky-backend/internal/auth"
	"github.com/tommygebru/kiekky-backend/internal/config"
	"github.com/tommygebru/kiekky-backend/internal/messaging"
	"github.com/tommygebru/kiekky-backend/internal/notification"
	"github.com/tommygebru/kiekky-backend/internal/posts"
	"github.com/tommygebru/kiekky-backend/internal/stories"
	"github.com/tommygebru/kiekky-backend/internal/user"
	"github.com/tommygebru/kiekky-backend/pkg/database"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	log.Println("========================================")
	log.Println("🚀 Starting Kiekky Social Media API")
	log.Println("========================================")

	// 1. Load environment
	if err := godotenv.Load(); err != nil {
		log.Printf("⚠️  No .env file found, using environment variables")
	}

	// 2. Load configuration
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatal("❌ Configuration error:", err)
	}
	log.Println("✅ Configuration loaded")

	// 3. Connect to PostgreSQL
	log.Println("🗄️  Connecting to PostgreSQL...")
	db, err := database.NewPostgresDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("❌ Database connection failed:", err)
	}
	defer db.Close()
	log.Println("✅ Connected to PostgreSQL")

	// 4. Initialize Auth module
	log.Println("🔐 Initializing Auth...")
	authRepo := auth.NewPostgresRepository(db)
	authConfig := &auth.Config{
		JWTSecret:          cfg.JWTSecret,
		AccessTokenExpiry:  cfg.AccessTokenExpiry,
		RefreshTokenExpiry: cfg.RefreshTokenExpiry,
		BCryptCost:         cfg.BCryptCost,
	}
	authService := auth.NewService(authRepo, authConfig)
	authHandler := auth.NewHandler(authService)
	authMiddleware := auth.NewMiddleware(authService)
	log.Println("✅ Auth initialized")

	// 5. Initialize User module (with Follow system)
	log.Println("👤 Initializing User & Follow system...")
	userRepo := user.NewPostgresRepository(db)
	userService := user.NewService(userRepo)
	userHandler := user.NewHandler(userService)
	log.Println("✅ User & Follow system initialized")

	// 6. Initialize Posts module
	log.Println("📝 Initializing Posts...")
	postsRepo := posts.NewPostgresRepository(db)
	postsService := posts.NewService(postsRepo)
	postsHandler := posts.NewHandler(postsService)
	log.Println("✅ Posts initialized")

	// 7. Initialize Stories module
	log.Println("📸 Initializing Stories...")
	storiesRepo := stories.NewPostgresRepository(db)
	storiesService := stories.NewService(storiesRepo)
	storiesHandler := stories.NewHandler(storiesService)
	log.Println("✅ Stories initialized")

	// 8. Initialize Messaging module with WebSocket
	log.Println("💬 Initializing Messaging...")
	messagingHub := messaging.NewHub()
	go messagingHub.Run()
	messagingRepo := messaging.NewPostgresRepository(db)
	messagingService := messaging.NewService(messagingRepo)
	messagingHandler := messaging.NewHandler(messagingService, messagingHub)
	log.Println("✅ Messaging initialized")

	// 9. Initialize Notification module
	log.Println("🔔 Initializing Notifications...")
	notificationRepo := notification.NewPostgresRepository(db)
	notificationService := notification.NewService(notificationRepo)
	notificationHandler := notification.NewHandler(notificationService)
	log.Println("✅ Notifications initialized")

	// 10. Setup routes
	log.Println("🛣️  Setting up routes...")
	router := mux.NewRouter()

	// Health check
	router.HandleFunc("/health", healthCheck).Methods("GET")
	router.HandleFunc("/api", apiInfo).Methods("GET")

	// Register routes
	authHandler.RegisterRoutes(router, authMiddleware)
	user.RegisterRoutes(router, userHandler, authMiddleware.Authenticate)
	posts.RegisterRoutes(router, postsHandler, authMiddleware.Authenticate)
	stories.RegisterRoutes(router, storiesHandler, authMiddleware.Authenticate)
	messaging.RegisterRoutes(router, messagingHandler, authMiddleware.Authenticate)
	notification.RegisterRoutes(router, notificationHandler, authMiddleware.Authenticate)

	// Static files for local uploads
	if !cfg.UseS3 {
		router.PathPrefix("/uploads/").Handler(
			http.StripPrefix("/uploads/", http.FileServer(http.Dir(cfg.LocalUploadDir))))
	}

	// Global middleware
	router.Use(loggingMiddleware)

	log.Println("✅ Routes registered")

	// Setup CORS using rs/cors package (proven to work)
	// Note: AllowCredentials with AllowedOrigins: "*" is not allowed per CORS spec
	// In production, specify exact origins. In development, we use AllowOriginFunc
	// You can also set ALLOWED_ORIGINS env var (comma-separated) to add more origins
	c := cors.New(cors.Options{
		AllowOriginFunc: func(origin string) bool {
			// In development, allow all origins
			if cfg.Environment != "production" {
				return true
			}
			// In production, add your allowed origins here
			allowedOrigins := []string{
				"https://kiekky.com",
				"https://www.kiekky.com",
				"https://app.kiekky.com",
				"https://kiekky.vercel.app",
				"https://kiekkyfront.vercel.app",
				"https://community-platform-core.vercel.app",
			}

			// Also check ALLOWED_ORIGINS environment variable
			if envOrigins := os.Getenv("ALLOWED_ORIGINS"); envOrigins != "" {
				for _, o := range strings.Split(envOrigins, ",") {
					allowedOrigins = append(allowedOrigins, strings.TrimSpace(o))
				}
			}

			for _, allowed := range allowedOrigins {
				if origin == allowed {
					return true
				}
			}
			return false
		},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Requested-With", "Origin"},
		ExposedHeaders:   []string{"Content-Length", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           86400,
		Debug:            cfg.Environment != "production",
	})
	handler := c.Handler(router)

	// 8. Start server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Println("========================================")
		log.Printf("🚀 Server running on http://localhost:%s", cfg.Port)
		log.Printf("🌍 Environment: %s", cfg.Environment)
		log.Println("========================================")

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("❌ Server error:", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("⚠️  Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("❌ Shutdown error:", err)
	}
	log.Println("✅ Server stopped")
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"healthy","service":"kiekky-api"}`))
}

func apiInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{
		"name": "Kiekky Social Media API",
		"version": "1.0.0",
		"endpoints": {
			"auth": "/api/v1/auth/*",
			"users": "/api/v1/users/*",
			"posts": "/api/v1/posts/*",
			"feed": "/api/v1/feed"
		}
	}`))
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.RequestURI, time.Since(start))
	})
}

//trigger a new deployment test
