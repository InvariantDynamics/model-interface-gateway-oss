package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/InvariantDynamics/model-interface-gateway-oss/core/pkg/mig"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
)

func main() {
	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	cfg, err := mig.ConfigFromEnv()
	if err != nil {
		log.Fatalf("invalid config: %v", err)
	}
	svc, err := mig.NewServiceWithOptions(mig.ServiceOptions{
		NATSURL:      cfg.NATSURL,
		AuditLogPath: cfg.AuditLogPath,
	})
	if err != nil {
		log.Fatalf("failed to initialize service: %v", err)
	}
	defer svc.Close()

	mux := http.NewServeMux()
	mig.RegisterHTTPRoutes(mux, svc)
	handler := mig.AuthMiddleware(cfg.Auth)(http.Handler(mux))
	if cfg.EnableMetrics {
		registry := prometheus.NewRegistry()
		metrics := mig.NewMetrics(registry)
		svc.SetMetrics(metrics)
		mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
		handler = metrics.Middleware(handler)
	}

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           withRequestLog(handler),
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("migd listening on %s (grpc=%s auth=%s metrics=%t nats=%s audit_log=%s)",
		cfg.Addr, displayOrNone(cfg.GRPCAddr), cfg.Auth.Mode, cfg.EnableMetrics, displayOrNone(cfg.NATSURL), displayOrNone(cfg.AuditLogPath))

	if cfg.NATSURL != "" && cfg.EnableNATSBinding {
		if _, err := svc.StartNATSBinding(); err != nil {
			log.Fatalf("failed to start NATS binding: %v", err)
		}
		log.Printf("migd NATS request/reply binding enabled on %s", cfg.NATSURL)
	}

	var grpcServer *grpc.Server
	if cfg.GRPCAddr != "" {
		grpcServer, _, err = mig.StartGRPCServer(rootCtx, cfg.GRPCAddr, svc, cfg.Auth)
		if err != nil {
			log.Fatalf("failed to start gRPC server: %v", err)
		}
		log.Printf("migd gRPC listening on %s", cfg.GRPCAddr)
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen failed: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	rootCancel()

	if grpcServer != nil {
		grpcServer.GracefulStop()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}

func withRequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func displayOrNone(value string) string {
	if value == "" {
		return "<none>"
	}
	return value
}
