// exporter_exporter is a reverse proxy for Prometheus exporters.
// It allows you to expose multiple exporters through a single endpoint,
// with support for authentication, TLS, and module-based routing.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/common/log"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	var (
		cfgFile         = flag.String("config.file", "expexp.yaml", "Path to configuration file.")
		listenAddress   = flag.String("web.listen-address", ":9999", "Address to listen on for web interface and telemetry.")
		telemetryPath   = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")
		proxyPath       = flag.String("web.proxy-path", "/proxy", "Path under which to expose proxied metrics.")
		showVersion     = flag.Bool("version", false, "Print version information and exit.")
		readTimeout     = flag.Duration("web.read-timeout", 60*time.Second, "HTTP server read timeout.")
		writeTimeout    = flag.Duration("web.write-timeout", 60*time.Second, "HTTP server write timeout.")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("exporter_exporter version=%s commit=%s date=%s\n", version, commit, date)
		os.Exit(0)
	}

	log.Infof("Starting exporter_exporter version=%s commit=%s date=%s", version, commit, date)

	cfg, err := loadConfig(*cfgFile)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc(*proxyPath, cfg.proxyHandler)
	mux.HandleFunc(*telemetryPath, cfg.metricsHandler)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html>
<head><title>Exporter Exporter</title></head>
<body>
<h1>Exporter Exporter</h1>
<p><a href=%q>Metrics</a></p>
<p><a href=%q>Proxy</a></p>
</body>
</html>`, *telemetryPath, *proxyPath)
	})

	srv := &http.Server{
		Addr:         *listenAddress,
		Handler:      mux,
		ReadTimeout:  *readTimeout,
		WriteTimeout: *writeTimeout,
	}

	go func() {
		log.Infof("Listening on %s", *listenAddress)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error starting HTTP server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Info("Server exited")
}
