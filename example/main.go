package main

import (
	"database/sql"
	"net/http"
	"time"

	obs "github.com/Assistencia-Familiar-Francana/go-observability"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/lib/pq"
)

func main() {
	// Initialize observability stack
	stack := obs.NewStack("example-service", true) // service name, debug mode
	logger := stack.Logger()

	logger.Info().Msg("Starting example service")

	// Setup database (example)
	db, err := sql.Open("postgres", "postgres://localhost/example?sslmode=disable")
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()

	// Setup router with observability middleware
	r := chi.NewRouter()

	// Middleware order matters!
	r.Use(middleware.RequestID)            // Generate request ID
	r.Use(obs.TraceMiddleware())           // Propagate trace context (X-Request-ID, X-Trace-ID)
	r.Use(stack.MetricsMiddleware())       // Collect Prometheus metrics
	r.Use(stack.LoggingMiddleware())       // Log requests with trace context
	r.Use(middleware.Recoverer)            // Recover from panics
	r.Use(middleware.Timeout(60 * time.Second))

	// Health endpoints
	r.Get("/healthz", obs.LivenessHandler())
	r.Get("/readyz", obs.ReadinessHandler(
		obs.DatabaseChecker(db),
		// obs.RedisChecker(redisClient), // Add if using Redis
	))

	// Metrics endpoint
	r.Handle("/metrics", obs.MetricsHandler())

	// Example API routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/hello", helloHandler)
		r.Get("/users/{id}", getUserHandler)
		r.Post("/users", createUserHandler)
	})

	// Start server
	addr := ":8080"
	logger.Info().Str("address", addr).Msg("Server starting")

	if err := http.ListenAndServe(addr, r); err != nil {
		logger.Fatal().Err(err).Msg("Server failed")
	}
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	// Get logger from context (includes trace IDs)
	logger := obs.LoggerFromContext(r.Context())

	logger.Info().Msg("Hello endpoint called")

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"message":"Hello, World!"}`))
}

func getUserHandler(w http.ResponseWriter, r *http.Request) {
	logger := obs.LoggerFromContext(r.Context())
	userID := chi.URLParam(r, "id")

	logger.Info().Str("user_id", userID).Msg("Fetching user")

	// Simulate database query
	time.Sleep(10 * time.Millisecond)

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"id":"` + userID + `","name":"John Doe"}`))
}

func createUserHandler(w http.ResponseWriter, r *http.Request) {
	logger := obs.LoggerFromContext(r.Context())

	logger.Info().Msg("Creating user")

	// Simulate processing
	time.Sleep(20 * time.Millisecond)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(`{"id":"123","name":"New User"}`))
}
