package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sb-module/internal/config"
	"sb-module/internal/database"
	"sb-module/internal/handlers"
	"sb-module/internal/middleware"
	"sb-module/pkg/logger"

	"github.com/gorilla/mux"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	logLevel := "info"
	if cfg.DebugMode {
		logLevel = "debug"
	}
	if cfg.IsProduction() {
		logLevel = "warn"
	}

	log := logger.New(logLevel)
	log.Info("Starting application", "environment", cfg.Environment, "debug", cfg.DebugMode)

	db, err := database.Connect(cfg.DatabaseURL, cfg.MaxConnections)
	if err != nil {
		log.Fatal("Failed to connect to database", "error", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Error("Error closing database connection", "error", err)
		}
	}()

	log.Info("Database connected successfully")

	if err := database.Ping(db); err != nil {
		log.Fatal("Database ping failed", "error", err)
	}

	healthHandler := handlers.NewHealthHandler(log, db)
	productHandler := handlers.NewProductHandler(db, log)
	cartHandler := handlers.NewCartHandler(db, log)
	orderHandler := handlers.NewOrderHandler(db, log)
	paymentHandler := handlers.NewPaymentHandler(db, log, cfg)

	router := mux.NewRouter()

	router.Use(middleware.Logging(log))
	router.Use(middleware.CORS(cfg))
	router.Use(middleware.Security())
	router.Use(middleware.RateLimiting())

	if cfg.Kong.InternalAuth != "" {
		router.Use(middleware.KongAuth(cfg.Kong.InternalAuth, cfg.Kong.AllowedIPs))
	}

	setupRoutes(router, healthHandler, productHandler, cartHandler, orderHandler, paymentHandler, cfg)

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  cfg.Timeout,
		WriteTimeout: cfg.Timeout,
		IdleTimeout:  cfg.Timeout * 2,
	}

	go func() {
		log.Info("Server starting", "port", cfg.Port, "environment", cfg.Environment)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Server failed to start", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Server shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown", "error", err)
	}

	log.Info("Server exited")
}

func setupRoutes(
	router *mux.Router,
	healthHandler *handlers.HealthHandler,
	productHandler *handlers.ProductHandler,
	cartHandler *handlers.CartHandler,
	orderHandler *handlers.OrderHandler,
	paymentHandler *handlers.PaymentHandler,
	cfg *config.Config,
) {
	api := router.PathPrefix("/api/v1").Subrouter()

	api.HandleFunc("/health", healthHandler.Check).Methods("GET")
	api.HandleFunc("/health/db", healthHandler.CheckDatabase).Methods("GET")
	api.HandleFunc("/health/ready", healthHandler.CheckReadiness).Methods("GET")

	api.HandleFunc("/products", productHandler.GetAll).Methods("GET")
	api.HandleFunc("/products/{id}", productHandler.GetByID).Methods("GET")
	api.HandleFunc("/products/category/{category}", productHandler.GetByCategory).Methods("GET")
	api.HandleFunc("/products/search", productHandler.Search).Methods("GET")
	api.HandleFunc("/categories", productHandler.GetCategories).Methods("GET")

	adminRoutes := api.PathPrefix("/admin").Subrouter()
	adminRoutes.Use(middleware.Auth(cfg.JWT.Secret))
	adminRoutes.Use(middleware.RequireRole("admin"))
	adminRoutes.HandleFunc("/products", productHandler.Create).Methods("POST")
	adminRoutes.HandleFunc("/products/{id}", productHandler.Update).Methods("PUT")
	adminRoutes.HandleFunc("/products/{id}", productHandler.Delete).Methods("DELETE")

	cartRoutes := api.PathPrefix("/cart").Subrouter()
	cartRoutes.Use(middleware.Auth(cfg.JWT.Secret))
	cartRoutes.HandleFunc("", cartHandler.Get).Methods("GET")
	cartRoutes.HandleFunc("", cartHandler.AddItem).Methods("POST")
	cartRoutes.HandleFunc("/items/{id}", cartHandler.UpdateItem).Methods("PUT")
	cartRoutes.HandleFunc("/items/{id}", cartHandler.RemoveItem).Methods("DELETE")
	cartRoutes.HandleFunc("/clear", cartHandler.Clear).Methods("DELETE")

	orderRoutes := api.PathPrefix("/orders").Subrouter()
	orderRoutes.Use(middleware.Auth(cfg.JWT.Secret))
	orderRoutes.HandleFunc("", orderHandler.Create).Methods("POST")
	orderRoutes.HandleFunc("", orderHandler.GetUserOrders).Methods("GET")
	orderRoutes.HandleFunc("/{id}", orderHandler.GetByID).Methods("GET")
	orderRoutes.HandleFunc("/{id}/cancel", orderHandler.Cancel).Methods("POST")

	paymentRoutes := api.PathPrefix("/payments").Subrouter()

	paymentRoutes.HandleFunc("", paymentHandler.CreatePayment).Methods("POST").Handler(
		middleware.Auth(cfg.JWT.Secret)(http.HandlerFunc(paymentHandler.CreatePayment)),
	)
	paymentRoutes.HandleFunc("/{id}/status", paymentHandler.GetPaymentStatus).Methods("GET").Handler(
		middleware.Auth(cfg.JWT.Secret)(http.HandlerFunc(paymentHandler.GetPaymentStatus)),
	)

	api.HandleFunc("/payments/callback", paymentHandler.HandleCallback).Methods("POST")

	api.HandleFunc("/payments/success", paymentHandler.HandleSuccess).Methods("GET")
	api.HandleFunc("/payments/cancel", paymentHandler.HandleCancel).Methods("GET")

	authRoutes := api.PathPrefix("/auth").Subrouter()

	api.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"message":     "Welcome to the Shockbliss API v1",
			"version":     "1.0.0",
			"environment": cfg.Environment,
			"timestamp":   time.Now().UTC().Format(time.RFC3339),
		}

		if !cfg.IsProduction() {
			response["debug"] = cfg.DebugMode
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"message": "%s",
			"version": "%s",
			"environment": "%s",
			"timestamp": "%s"
		}`,
			response["message"],
			response["version"],
			response["environment"],
			response["timestamp"],
		)
	}).Methods("GET")

	api.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "endpoint not found", "path": "` + r.URL.Path + `"}`))
	})
}
