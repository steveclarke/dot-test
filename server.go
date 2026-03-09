package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
)

// Server runs the embedded DNS resolver and HTTP reverse proxy.
type Server struct {
	cfg    Config
	routes map[string]int // sanitized app name -> port
	mu     sync.RWMutex
}

func newServer(cfg Config) *Server {
	return &Server{
		cfg:    cfg,
		routes: make(map[string]int),
	}
}

// loadRoutes reads the portmap file and populates the route table.
func (s *Server) loadRoutes() {
	mappings := loadPortmap(s.cfg.PortmapFile)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.routes = make(map[string]int)
	for _, pm := range mappings {
		name := sanitizeName(pm.App)
		s.routes[name] = pm.Port
	}
}

// --- DNS server (pure stdlib, zero dependencies) ---

func (s *Server) startDNS() error {
	addr := fmt.Sprintf("127.0.0.1:%d", s.cfg.DNSPort)
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		return fmt.Errorf("dns: %w", err)
	}
	defer conn.Close()
	log.Printf("DNS listening on %s (*.test -> 127.0.0.1)", addr)

	buf := make([]byte, 512)
	for {
		n, remote, err := conn.ReadFrom(buf)
		if err != nil {
			continue
		}
		resp := s.handleDNS(buf[:n])
		if resp != nil {
			conn.WriteTo(resp, remote)
		}
	}
}

func (s *Server) handleDNS(query []byte) []byte {
	if len(query) < 12 {
		return nil
	}

	id := binary.BigEndian.Uint16(query[0:2])
	qdcount := binary.BigEndian.Uint16(query[4:6])
	if qdcount == 0 {
		return nil
	}

	// Parse first question
	name, qEnd := readDNSName(query, 12)
	if qEnd < 0 || qEnd+4 > len(query) {
		return nil
	}
	qtype := binary.BigEndian.Uint16(query[qEnd : qEnd+2])
	qclass := binary.BigEndian.Uint16(query[qEnd+2 : qEnd+4])
	questionBytes := query[12 : qEnd+4]

	isMatch := qtype == 1 && qclass == 1 && strings.HasSuffix(strings.ToLower(name), ".test.")

	// Build response
	resp := make([]byte, 0, 128)

	// Header (12 bytes)
	hdr := make([]byte, 12)
	binary.BigEndian.PutUint16(hdr[0:2], id)
	if isMatch {
		binary.BigEndian.PutUint16(hdr[2:4], 0x8400) // QR=1, AA=1, RCODE=0
		binary.BigEndian.PutUint16(hdr[6:8], 1)       // ANCOUNT=1
	} else {
		binary.BigEndian.PutUint16(hdr[2:4], 0x8403) // QR=1, AA=1, RCODE=3 (NXDOMAIN)
	}
	binary.BigEndian.PutUint16(hdr[4:6], qdcount) // QDCOUNT
	resp = append(resp, hdr...)

	// Question section (echo back)
	resp = append(resp, questionBytes...)

	// Answer section
	if isMatch {
		// Name pointer to question at offset 12
		resp = append(resp, 0xC0, 0x0C)
		// Type A (1), Class IN (1)
		resp = append(resp, 0, 1, 0, 1)
		// TTL = 60
		ttl := make([]byte, 4)
		binary.BigEndian.PutUint32(ttl, 60)
		resp = append(resp, ttl...)
		// RDLENGTH = 4, RDATA = 127.0.0.1
		resp = append(resp, 0, 4, 127, 0, 0, 1)
	}

	return resp
}

// readDNSName parses a DNS name from wire format, handling label compression.
func readDNSName(buf []byte, offset int) (string, int) {
	var parts []string
	pos := offset
	jumped := false
	retPos := -1

	for pos < len(buf) {
		length := int(buf[pos])
		if length == 0 {
			if !jumped {
				retPos = pos + 1
			}
			break
		}
		// Pointer (compression)
		if length&0xC0 == 0xC0 {
			if pos+1 >= len(buf) {
				return "", -1
			}
			if !jumped {
				retPos = pos + 2
			}
			pos = int(binary.BigEndian.Uint16(buf[pos:pos+2]) & 0x3FFF)
			jumped = true
			continue
		}
		pos++
		if pos+length > len(buf) {
			return "", -1
		}
		parts = append(parts, string(buf[pos:pos+length]))
		pos += length
	}

	if retPos < 0 {
		retPos = pos + 1
	}
	return strings.Join(parts, ".") + ".", retPos
}

// --- HTTP reverse proxy ---

func (s *Server) startProxy() error {
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			host := strings.Split(req.Host, ":")[0]
			name := strings.TrimSuffix(host, ".test")
			s.mu.RLock()
			port, ok := s.routes[name]
			s.mu.RUnlock()

			if ok {
				target, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", port))
				req.URL.Scheme = target.Scheme
				req.URL.Host = target.Host
			}
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			host := strings.Split(r.Host, ":")[0]
			name := strings.TrimSuffix(host, ".test")
			w.WriteHeader(http.StatusBadGateway)
			fmt.Fprintf(w, "dot-test: %s is not running\n\nStart it with: cd ~/projects/%s && bin/dev\n", host, name)
		},
	}

	addr := fmt.Sprintf(":%d", s.cfg.ProxyPort)
	srv := &http.Server{Addr: addr, Handler: proxy}
	log.Printf("HTTP proxy listening on %s", addr)
	return srv.ListenAndServe()
}

// run starts both DNS and HTTP servers.
func (s *Server) run() error {
	s.loadRoutes()

	errCh := make(chan error, 2)
	go func() { errCh <- s.startDNS() }()
	go func() { errCh <- s.startProxy() }()

	return <-errCh
}
