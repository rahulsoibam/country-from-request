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

var xForwardedFor = http.CanonicalHeaderKey("X-Forwarded-For")
var xRealIP = http.CanonicalHeaderKey("X-Real-IP")

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
			for i := len(ips) - 1; i >= 0; i-- {
				ip := strings.TrimSpace(ips[i])
				realIP := net.ParseIP(ip)
				if !realIP.IsGlobalUnicast() || isPrivateIP(realIP) {
					// bad address, go next
					continue
				}
				return ip
			}
		}
	}

	return strings.Split(r.RemoteAddr, ":")[0]
	// log.Println(r.Header.Get(http.CanonicalHeaderKey("X-Forwarded-For")))
	// log.Println(r.Header.Get(http.CanonicalHeaderKey("X-Real-IP")))
	// for _, h := range []string{"X-Forwarded-For", "X-Real-IP"} {
	// 	addresses := strings.Split(r.Header.Get(http.CanonicalHeaderKey(h)), ",")
	// 	log.Print("addresses:")
	// 	log.Println(addresses)
	// 	// go from right to left until we get a public address that will be the address right before our proxy or load balancer.
	// 	for i := len(addresses) - 1; i >= 0; i-- {
	// 		// Headers can contain spaces, so strip them out
	// 		log.Printf("addresses[%d]:", i)
	// 		log.Println(addresses[i])
	// 		ip := strings.TrimSpace(addresses[i])
	// 		log.Print("ip:")
	// 		log.Println(ip)
	// 		realIP := net.ParseIP(ip)
	// 		log.Print("realIP:")
	// 		log.Println(realIP)
	// 		log.Print("IsGlobalUnicast")
	// 		log.Println(realIP.IsGlobalUnicast())
	// 		log.Print("isPrivateIP")
	// 		log.Println(isPrivateIP(realIP))

	// 		if !realIP.IsGlobalUnicast() || isPrivateIP(realIP) {
	// 			// bad address, go to next
	// 			continue
	// 		}
	// 		return ip
	// 	}
	// }
	// return ""
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}
