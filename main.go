package main

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
)

var privateIPBlocks []*net.IPNet

func init() {
	for _, cidr := range []string{
		"127.0.0.0/8",    // IPv4 loopback
		"10.0.0.0/8",     // RFC1918
		"172.16.0.0/12",  // RFC1918
		"192.168.0.0/16", // RFC1918
		"::1/128",        // IPv6 loopback
		"fe80::/10",      // IPv6 link-local
		"fc00::/7",       // Unique local address
	} {
		_, block, _ := net.ParseCIDR(cidr)
		privateIPBlocks = append(privateIPBlocks, block)
	}
}

func main() {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Set a timeout value on the request context (ctx), that will signal through
	// ctx.Done() that the request has timed out and further processing should be stopped
	r.Use(middleware.Timeout(120 * time.Second))

	r.Get("/", returnIPAddress)
	log.Fatal(http.ListenAndServe(":8082", r))
}

func returnIPAddress(w http.ResponseWriter, r *http.Request) {
	ip := getIPAddress(r)
	respondWithJSON(w, http.StatusOK, ip)
}

func isPrivateIP(ip net.IP) bool {
	for _, block := range privateIPBlocks {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

func getIPAddress(r *http.Request) string {
	for _, h := range []string{"X-Forwarded-For", "X-Real-Ip"} {
		addresses := strings.Split(r.Header.Get(h), ",")
		// go from right to left until we get a public address that will be the address right before our proxy or load balancer.
		for i := len(addresses) - 1; i >= 0; i-- {
			// Headers can contain spaces, so strip them out
			ip := strings.TrimSpace(addresses[i])
			realIP := net.ParseIP(ip)

			if !realIP.IsGlobalUnicast() || isPrivateIP(realIP) {
				// bad address, go to next
				continue
			}
			return ip
		}
	}
	return ""
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}