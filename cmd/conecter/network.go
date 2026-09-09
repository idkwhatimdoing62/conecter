package main

import (
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type lookupRate struct {
	started time.Time
	count   int
}

var lookupMu sync.Mutex
var lookups = map[string]lookupRate{}

func allowLookup(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || host == "" {
		host = r.RemoteAddr
	}
	now := time.Now()
	lookupMu.Lock()
	defer lookupMu.Unlock()
	for key, entry := range lookups {
		if now.Sub(entry.started) >= time.Minute {
			delete(lookups, key)
		}
	}
	entry := lookups[host]
	if now.Sub(entry.started) >= time.Minute {
		entry = lookupRate{started: now}
	}
	entry.count++
	lookups[host] = entry
	return entry.count <= 60
}

func networkInfo(w http.ResponseWriter, r *http.Request) {
	ip := "127.0.0.1"
	best := 9
	if ifaces, err := net.Interfaces(); err == nil {
		for _, iface := range ifaces {
			name := strings.ToLower(iface.Name)
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 || strings.Contains(name, "tun") || strings.Contains(name, "vpn") || strings.Contains(name, "virtual") {
				continue
			}
			if addrs, err := iface.Addrs(); err == nil {
				for _, addr := range addrs {
					if n, ok := addr.(*net.IPNet); ok && n.IP.To4() != nil && !n.IP.IsLoopback() {
						candidate := n.IP.String()
						rank := 3
						if strings.HasPrefix(candidate, "192.168.") {
							rank = 0
						} else if strings.HasPrefix(candidate, "10.") {
							rank = 1
						} else if strings.HasPrefix(candidate, "172.") {
							rank = 2
						}
						if rank < best {
							ip = candidate
							best = rank
						}
					}
				}
			}
		}
	}
	jsonOut(w, map[string]string{"ip": ip}, http.StatusOK)
}

func securityHeaders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com https://cdn.jsdelivr.net; font-src 'self' https://fonts.gstatic.com https://cdn.jsdelivr.net; img-src 'self' data: http:; connect-src 'self' http: ws:; frame-ancestors 'none'; base-uri 'self'")
	origin := r.Header.Get("Origin")
	if origin != "" {
		if parsed, err := url.Parse(origin); err == nil {
			host := r.Host
			if h, _, err := net.SplitHostPort(r.Host); err == nil {
				host = h
			}
			if parsed.Hostname() == host {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
			}
		}
	}
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}
