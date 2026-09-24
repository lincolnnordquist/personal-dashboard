package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"dashboard/cache"
	"dashboard/config"
	"dashboard/db"
	"dashboard/graph"
	"dashboard/widgets"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool, cfg); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	repo := db.NewWidgetRepo(pool)
	store := cache.NewPostgresStore(pool)
	weather := widgets.NewWeatherClient()
	reddit := widgets.NewRedditClient(cfg.RedditClientID, cfg.RedditClientSecret)
	sports := widgets.NewSportsClient()
	youtube := widgets.NewYouTubeClient(cfg.YouTubeAPIKey)
	if cfg.YouTubeAPIKey == "" {
		log.Printf("youtube: YOUTUBE_API_KEY not set; the YouTube widget will show an error")
	}
	docker, err := widgets.NewDockerClient()
	if err != nil {
		log.Printf("docker: %v (the Docker widget will show an error)", err)
	}
	if !reddit.UsesOAuth() {
		log.Printf("reddit: no API credentials, using RSS feeds (no scores or comment counts)")
	}

	go every(ctx, widgets.WeatherTTL, refreshWeather(repo, store, weather))
	go every(ctx, widgets.RedditTTL, refreshReddit(repo, store, reddit))
	go every(ctx, widgets.SportsTTL, refreshSports(repo, store, sports))
	go every(ctx, widgets.YouTubeTTL, refreshYouTube(repo, store, youtube))

	resolver := &graph.Resolver{
		WidgetRepo: repo,
		Cache:      store,
		Weather:    weather,
		Reddit:     reddit,
		Sports:     sports,
		YouTube:    youtube,
		Docker:     docker,
		Now:        time.Now,
	}
	gql := handler.New(graph.NewExecutableSchema(graph.Config{Resolvers: resolver}))
	gql.AddTransport(transport.GET{})
	gql.AddTransport(transport.POST{})
	gql.Use(extension.Introspection{})

	mux := http.NewServeMux()
	mux.Handle("/graphql", gql)
	mux.Handle("GET /playground", playground.Handler("Dashboard GraphQL", "/graphql"))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: mux}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	log.Printf("listening on :%s", cfg.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
