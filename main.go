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
	var values []string
	values = append(values, "X-Forwarded-For")
	values = append(values, "X-Real-IP")

	for _, h := range values {
		if xff := r.Header.Get(http.CanonicalHeaderKey(h)); xff != "" {
			ips := strings.Split(xff, ", ")
			log.Print("ips:")
			log.Println(ips)
			for i := len(ips) - 1; i >= 0; i-- {
				ip := strings.TrimSpace(ips[i])
				realIP := net.ParseIP(ip)
				log.Print("realIP:")
				log.Println(realIP)

				log.Print("IsGlobalUnicast")
				log.Println(realIP.IsGlobalUnicast())
				log.Print("isPrivateIP")
				log.Println(isPrivateIP(realIP))
				if isPrivateIP(realIP) || !realIP.IsGlobalUnicast() {
					// bad address, go next
					log.Println("Bad address...")
					continue
				} else {
					log.Println("Returning...")
					return ip
				}
			}
		}
	}
	ipStr := strings.Split(r.RemoteAddr, ":")[0]
	log.Println("ipStr")
	rIP := net.ParseIP(ipStr)
	log.Println(rIP)

	if isPrivateIP(rIP) || !rIP.IsGlobalUnicast() {
		log.Println("Inside if block")
		return ""
	}
	return rIP.String()
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}
