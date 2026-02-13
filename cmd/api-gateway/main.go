package main

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

type Gateway struct {
	subscriptionProxy *httputil.ReverseProxy
	orchestratorProxy *httputil.ReverseProxy
	bpasProxy         *httputil.ReverseProxy
	schedulerProxy    *httputil.ReverseProxy
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func buildReverseProxy(rawURL, serviceName string) *httputil.ReverseProxy {
	target, err := url.Parse(rawURL)
	if err != nil {
		log.Fatalf("invalid %s URL %q: %v", serviceName, rawURL, err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Header.Set("X-Forwarded-Host", req.Host)
		req.Header.Set("X-Gateway-Service", serviceName)
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("proxy error (%s): %v", serviceName, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "upstream_unavailable",
			"service": serviceName,
		})
	}
	return proxy
}

func (g *Gateway) proxySubscription(w http.ResponseWriter, r *http.Request) {
	g.subscriptionProxy.ServeHTTP(w, r)
}

func (g *Gateway) proxyOrchestrator(w http.ResponseWriter, r *http.Request) {
	g.orchestratorProxy.ServeHTTP(w, r)
}

func (g *Gateway) proxyBPAS(w http.ResponseWriter, r *http.Request) {
	g.bpasProxy.ServeHTTP(w, r)
}

func (g *Gateway) proxyScheduler(w http.ResponseWriter, r *http.Request) {
	g.schedulerProxy.ServeHTTP(w, r)
}

func (g *Gateway) proxyWebsocket(w http.ResponseWriter, r *http.Request) {
	g.orchestratorProxy.ServeHTTP(w, r)
}

func (g *Gateway) createRefundAlias(w http.ResponseWriter, r *http.Request) {
	// Backward-compatible alias: POST /refunds -> POST /orchestrator/refund
	r.URL.Path = "/orchestrator/refund"
	g.orchestratorProxy.ServeHTTP(w, r)
}

func main() {
	subscriptionURL := envOrDefault("SUBSCRIPTION_SERVICE_URL", "http://localhost:8002")
	orchestratorURL := envOrDefault("PAYMENT_ORCHESTRATOR_URL", "http://localhost:8001")
	bpasURL := envOrDefault("BPAS_SERVICE_URL", "http://localhost:8003")
	schedulerURL := envOrDefault("MIT_SCHEDULER_URL", "http://localhost:8004")
	port := envOrDefault("PORT", "8080")

	gateway := &Gateway{
		subscriptionProxy: buildReverseProxy(subscriptionURL, "subscription-service"),
		orchestratorProxy: buildReverseProxy(orchestratorURL, "payment-orchestrator"),
		bpasProxy:         buildReverseProxy(bpasURL, "bpas-service"),
		schedulerProxy:    buildReverseProxy(schedulerURL, "mit-scheduler"),
	}

	r := mux.NewRouter()

	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"service":   "api-gateway",
			"status":    "healthy",
			"timestamp": time.Now(),
			"routes": map[string]string{
				"subscriptions": "/subscriptions,/plans,/stats/subscriptions",
				"orchestrator":  "/orchestrator/*,/refunds,/stats/transactions,/stats/processors,/admin/stats,/ws",
				"bpas":          "/bpas/*,/rules/*",
				"scheduler":     "/scheduler/*",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}).Methods("GET")

	// Subscription-service routes
	r.PathPrefix("/subscriptions").HandlerFunc(gateway.proxySubscription)
	r.PathPrefix("/plans").HandlerFunc(gateway.proxySubscription)
	r.Path("/stats/subscriptions").HandlerFunc(gateway.proxySubscription)

	// Orchestrator routes
	r.PathPrefix("/orchestrator").HandlerFunc(gateway.proxyOrchestrator)
	r.Path("/refunds").Methods("POST").HandlerFunc(gateway.createRefundAlias)
	r.Path("/stats/transactions").HandlerFunc(gateway.proxyOrchestrator)
	r.Path("/stats/processors").HandlerFunc(gateway.proxyOrchestrator)
	r.Path("/admin/stats").HandlerFunc(gateway.proxyOrchestrator)
	r.PathPrefix("/ws").HandlerFunc(gateway.proxyWebsocket)

	// BPAS routes
	r.PathPrefix("/bpas").HandlerFunc(gateway.proxyBPAS)
	r.PathPrefix("/rules").HandlerFunc(gateway.proxyBPAS)

	// Scheduler routes
	r.PathPrefix("/scheduler").HandlerFunc(gateway.proxyScheduler)

	// Fallback
	r.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":   "route_not_found",
			"message": "use /health to inspect available routes",
			"path":    strings.TrimSpace(r.URL.Path),
		})
	})

	log.Printf("API Gateway starting on port %s", port)
	log.Printf("Upstreams: subscription=%s orchestrator=%s bpas=%s scheduler=%s",
		subscriptionURL, orchestratorURL, bpasURL, schedulerURL)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
