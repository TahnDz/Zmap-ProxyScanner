/*
	(c) Yariya
*/

package main

import (
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Proxy struct {
	ips            map[string]struct{}
	timeout        time.Duration
	maxHttpThreads int64

	openHttpThreads int64
	mu              sync.Mutex
	processingMu    sync.Mutex
	processing      map[string]bool
}

var Proxies = &Proxy{
	timeout:        time.Second * 5,
	maxHttpThreads: int64(config.HttpThreads),
	ips:            make(map[string]struct{}),
	processing:     make(map[string]bool),
}

func (p *Proxy) WorkerThread() {
	for {
		time.Sleep(time.Millisecond * 50)

		if atomic.LoadInt64(&p.openHttpThreads) >= int64(config.HttpThreads) {
			continue
		}

		p.mu.Lock()
		var proxyToCheck string
		var found bool

		for proxy := range p.ips {
			p.processingMu.Lock()
			if !p.processing[proxy] {
				p.processing[proxy] = true
				proxyToCheck = proxy
				found = true
				delete(p.ips, proxy)
				p.processingMu.Unlock()
				break
			}
			p.processingMu.Unlock()
		}
		p.mu.Unlock()

		if !found {
			continue
		}

		if strings.ToLower(config.ProxyType) == "http" {
			go p.CheckProxyHTTP(proxyToCheck)
		} else if strings.ToLower(config.ProxyType) == "socks4" {
			go p.CheckProxySocks4(proxyToCheck)
		} else if strings.ToLower(config.ProxyType) == "socks5" {
			go p.CheckProxySocks5(proxyToCheck)
		} else {
			log.Fatalln("invalid ProxyType")
		}
	}
}

func (p *Proxy) testRawSocketViaHTTPProxy(proxyIP string, proxyPort int) bool {
	timeout := time.Duration(config.RawSocketTest.Timeout)
	if timeout == 0 {
		timeout = time.Duration(config.Timeout.HttpTimeout)
	}
	timeout = timeout * time.Second

	// Connect to HTTP proxy
	proxyAddr := fmt.Sprintf("%s:%d", proxyIP, proxyPort)
	conn, err := net.DialTimeout("tcp", proxyAddr, timeout)
	if err != nil {
		atomic.AddUint64(&proxyErr, 1)
		if strings.Contains(err.Error(), "timeout") {
			atomic.AddUint64(&timeoutErr, 1)
		}
		return false
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(timeout))

	// Send HTTP CONNECT request to establish tunnel
	targetAddr := fmt.Sprintf("%s:%d", config.RawSocketTest.TargetHost, config.RawSocketTest.TargetPort)
	connectRequest := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", targetAddr, targetAddr)

	_, err = conn.Write([]byte(connectRequest))
	if err != nil {
		atomic.AddUint64(&proxyErr, 1)
		return false
	}

	// Read HTTP CONNECT response
	buffer := make([]byte, 4096)
	n, err := conn.Read(buffer)
	if err != nil {
		atomic.AddUint64(&proxyErr, 1)
		if strings.Contains(err.Error(), "timeout") {
			atomic.AddUint64(&timeoutErr, 1)
		}
		return false
	}

	response := string(buffer[:n])
	if !strings.Contains(response, "200") && !strings.Contains(response, "Connection established") {
		atomic.AddUint64(&statusCodeErr, 1)
		return false
	}

	// Now we have a tunnel, send raw bytes
	if config.RawSocketTest.SendHex != "" {
		rawBytes, err := hex.DecodeString(config.RawSocketTest.SendHex)
		if err != nil {
			log.Printf("Invalid hex string: %v\n", err)
			atomic.AddUint64(&proxyErr, 1)
			return false
		}

		_, err = conn.Write(rawBytes)
		if err != nil {
			atomic.AddUint64(&proxyErr, 1)
			return false
		}
	}

	// Check if we expect a response
	if config.RawSocketTest.ExpectResponse {
		responseBuffer := make([]byte, 4096)
		conn.SetReadDeadline(time.Now().Add(timeout))
		
		n, err := conn.Read(responseBuffer)
		if err != nil {
			atomic.AddUint64(&proxyErr, 1)
			if strings.Contains(err.Error(), "timeout") {
				atomic.AddUint64(&timeoutErr, 1)
			}
			return false
		}

		// Check minimum response size
		if config.RawSocketTest.ResponseMinSize > 0 && n < config.RawSocketTest.ResponseMinSize {
			atomic.AddUint64(&statusCodeErr, 1)
			return false
		}
	}

	return true
}

func (p *Proxy) CheckProxyHTTP(proxy string) {
	atomic.AddInt64(&p.openHttpThreads, 1)
	defer func() {
		atomic.AddInt64(&p.openHttpThreads, -1)
		atomic.AddUint64(&checked, 1)
		p.processingMu.Lock()
		delete(p.processing, proxy)
		p.processingMu.Unlock()
	}()

	var err error
	var proxyPort = *port
	s := strings.Split(proxy, ":")
	if len(s) > 1 {
		proxyPort, err = strconv.Atoi(strings.TrimSpace(s[1]))
		if err != nil {
			log.Println(err)
			return
		}
	}

	// If raw socket test is enabled, use it instead of HTTP
	if config.RawSocketTest.Enabled {
		success := p.testRawSocketViaHTTPProxy(s[0], proxyPort)
		if success {
			if config.PrintIps.Enabled {
				go PrintProxy(s[0], proxyPort)
			}
			atomic.AddUint64(&success, 1)
			exporter.Add(fmt.Sprintf("%s:%d", s[0], proxyPort))
		}
		return
	}

	// Original HTTP checking logic
	proxyUrl, err := url.Parse(fmt.Sprintf("http://%s:%d", s[0], proxyPort))
	if err != nil {
		log.Println(err)
		return
	}

	tr := &http.Transport{
		Proxy: http.ProxyURL(proxyUrl),
		DialContext: (&net.Dialer{
			Timeout:   time.Second * time.Duration(config.Timeout.HttpTimeout),
			KeepAlive: time.Second,
			DualStack: true,
		}).DialContext,
	}

	client := http.Client{
		Timeout:   time.Second * time.Duration(config.Timeout.HttpTimeout),
		Transport: tr,
	}

	// Check all configured sites
	allPassed := true
	for _, site := range config.CheckSites {
		req, err := http.NewRequest("GET", site.URL, nil)
		if err != nil {
			log.Println(err)
			atomic.AddUint64(&proxyErr, 1)
			return
		}
		req.Header.Add("User-Agent", config.Headers.UserAgent)
		req.Header.Add("accept", config.Headers.Accept)

		res, err := client.Do(req)
		if err != nil {
			atomic.AddUint64(&proxyErr, 1)
			if strings.Contains(err.Error(), "timeout") {
				atomic.AddUint64(&timeoutErr, 1)
			}
			allPassed = false
			break
		}

		body, _ := io.ReadAll(res.Body)
		res.Body.Close()

		// Check status code
		expectedStatus := site.StatusCode
		if expectedStatus == 0 {
			expectedStatus = 200 // default
		}

		if res.StatusCode != expectedStatus {
			atomic.AddUint64(&statusCodeErr, 1)
			allPassed = false
			break
		}

		// Check response contains
		if len(site.ResponseContains) > 0 {
			bodyStr := string(body)
			for _, mustContain := range site.ResponseContains {
				if !strings.Contains(bodyStr, mustContain) {
					atomic.AddUint64(&statusCodeErr, 1)
					allPassed = false
					break
				}
			}
			if !allPassed {
				break
			}
		}
	}

	if allPassed {
		if config.PrintIps.Enabled {
			go PrintProxy(s[0], proxyPort)
		}
		atomic.AddUint64(&success, 1)
		exporter.Add(fmt.Sprintf("%s:%d", s[0], proxyPort))
	}
}

func (p *Proxy) testRawSocketViaSocks4(proxyIP string, proxyPort int) bool {
	timeout := time.Duration(config.RawSocketTest.Timeout)
	if timeout == 0 {
		timeout = time.Duration(config.Timeout.Socks4Timeout)
	}
	timeout = timeout * time.Second

	// Connect to SOCKS4 proxy
	conn, err := net.DialTimeout(config.RawSocketTest.Protocol, fmt.Sprintf("%s:%d", proxyIP, proxyPort), timeout)
	if err != nil {
		atomic.AddUint64(&proxyErr, 1)
		if strings.Contains(err.Error(), "timeout") {
			atomic.AddUint64(&timeoutErr, 1)
		}
		return false
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(timeout))

	// Resolve target hostname to IP for SOCKS4
	targetIPs, err := net.LookupIP(config.RawSocketTest.TargetHost)
	if err != nil || len(targetIPs) == 0 {
		atomic.AddUint64(&proxyErr, 1)
		return false
	}

	targetIP := targetIPs[0].To4()
	if targetIP == nil {
		log.Println("SOCKS4 only supports IPv4")
		atomic.AddUint64(&proxyErr, 1)
		return false
	}

	// Build SOCKS4 CONNECT request
	targetPort := config.RawSocketTest.TargetPort
	request := []byte{
		0x04, // SOCKS version 4
		0x01, // CONNECT command
		byte(targetPort >> 8), byte(targetPort & 0xFF), // Port (big-endian)
		targetIP[0], targetIP[1], targetIP[2], targetIP[3], // IP address
		0x00, // NULL terminator for user ID
	}

	_, err = conn.Write(request)
	if err != nil {
		atomic.AddUint64(&proxyErr, 1)
		return false
	}

	// Read SOCKS4 response
	response := make([]byte, 8)
	_, err = io.ReadFull(conn, response)
	if err != nil {
		atomic.AddUint64(&proxyErr, 1)
		if strings.Contains(err.Error(), "timeout") {
			atomic.AddUint64(&timeoutErr, 1)
		}
		return false
	}

	// Check if connection was granted (0x5A = granted)
	if response[1] != 0x5A {
		atomic.AddUint64(&statusCodeErr, 1)
		return false
	}

	// Connection established, now send raw bytes
	if config.RawSocketTest.SendHex != "" {
		rawBytes, err := hex.DecodeString(config.RawSocketTest.SendHex)
		if err != nil {
			log.Printf("Invalid hex string: %v\n", err)
			atomic.AddUint64(&proxyErr, 1)
			return false
		}

		_, err = conn.Write(rawBytes)
		if err != nil {
			atomic.AddUint64(&proxyErr, 1)
			return false
		}
	}

	// Check if we expect a response
	if config.RawSocketTest.ExpectResponse {
		responseBuffer := make([]byte, 4096)
		conn.SetReadDeadline(time.Now().Add(timeout))
		
		n, err := conn.Read(responseBuffer)
		if err != nil {
			atomic.AddUint64(&proxyErr, 1)
			if strings.Contains(err.Error(), "timeout") {
				atomic.AddUint64(&timeoutErr, 1)
			}
			return false
		}

		// Check minimum response size
		if config.RawSocketTest.ResponseMinSize > 0 && n < config.RawSocketTest.ResponseMinSize {
			atomic.AddUint64(&statusCodeErr, 1)
			return false
		}
	}

	return true
}

func (p *Proxy) CheckProxySocks4(proxy string) {
	atomic.AddInt64(&p.openHttpThreads, 1)
	defer func() {
		atomic.AddInt64(&p.openHttpThreads, -1)
		atomic.AddUint64(&checked, 1)
		p.processingMu.Lock()
		delete(p.processing, proxy)
		p.processingMu.Unlock()
	}()

	var err error
	var proxyPort = *port
	s := strings.Split(proxy, ":")
	if len(s) > 1 {
		proxyPort, err = strconv.Atoi(strings.TrimSpace(s[1]))
		if err != nil {
			log.Println(err)
			return
		}
	}

	// If raw socket test is enabled, use custom target
	if config.RawSocketTest.Enabled {
		success := p.testRawSocketViaSocks4(s[0], proxyPort)
		if success {
			if config.PrintIps.Enabled {
				go PrintProxy(s[0], proxyPort)
			}
			atomic.AddUint64(&success, 1)
			exporter.Add(fmt.Sprintf("%s:%d", s[0], proxyPort))
		}
		return
	}

	// Original SOCKS4 test with 8.8.8.8
	timeout := time.Second * time.Duration(config.Timeout.Socks4Timeout)
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", s[0], proxyPort), timeout)
	if err != nil {
		atomic.AddUint64(&proxyErr, 1)
		if strings.Contains(err.Error(), "timeout") {
			atomic.AddUint64(&timeoutErr, 1)
		}
		return
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(timeout))

	// Send SOCKS4 connect request to 8.8.8.8:80
	request := []byte{
		0x04,       // SOCKS version 4
		0x01,       // CONNECT command
		0x00, 0x50, // Port 80 (HTTP)
		0x08, 0x08, 0x08, 0x08, // IP 8.8.8.8
		0x00, // NULL terminator for user ID
	}

	_, err = conn.Write(request)
	if err != nil {
		atomic.AddUint64(&proxyErr, 1)
		return
	}

	// Read response
	response := make([]byte, 8)
	_, err = io.ReadFull(conn, response)
	if err != nil {
		atomic.AddUint64(&proxyErr, 1)
		if strings.Contains(err.Error(), "timeout") {
			atomic.AddUint64(&timeoutErr, 1)
		}
		return
	}

	// Check if connection was granted
	if response[1] != 0x5A {
		atomic.AddUint64(&statusCodeErr, 1)
		return
	}

	// Success
	if config.PrintIps.Enabled {
		go PrintProxy(s[0], proxyPort)
	}
	atomic.AddUint64(&success, 1)
	exporter.Add(fmt.Sprintf("%s:%d", s[0], proxyPort))
}

func (p *Proxy) testRawSocketViaSocks5(proxyIP string, proxyPort int) bool {
	timeout := time.Duration(config.RawSocketTest.Timeout)
	if timeout == 0 {
		timeout = time.Duration(config.Timeout.Socks5Timeout)
	}
	timeout = timeout * time.Second

	// Connect to SOCKS5 proxy
	conn, err := net.DialTimeout(config.RawSocketTest.Protocol, fmt.Sprintf("%s:%d", proxyIP, proxyPort), timeout)
	if err != nil {
		atomic.AddUint64(&proxyErr, 1)
		if strings.Contains(err.Error(), "timeout") {
			atomic.AddUint64(&timeoutErr, 1)
		}
		return false
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(timeout))

	// SOCKS5 handshake - no authentication
	handshake := []byte{0x05, 0x01, 0x00}
	_, err = conn.Write(handshake)
	if err != nil {
		atomic.AddUint64(&proxyErr, 1)
		return false
	}

	// Read handshake response
	handshakeResp := make([]byte, 2)
	_, err = io.ReadFull(conn, handshakeResp)
	if err != nil {
		atomic.AddUint64(&proxyErr, 1)
		return false
	}

	if handshakeResp[0] != 0x05 || handshakeResp[1] != 0x00 {
		atomic.AddUint64(&proxyErr, 1)
		return false
	}

	// Build CONNECT request
	targetPort := config.RawSocketTest.TargetPort
	var connectReq []byte

	// Try to parse as IP first
	targetIP := net.ParseIP(config.RawSocketTest.TargetHost)
	if targetIP != nil && targetIP.To4() != nil {
		// IPv4 address
		ip4 := targetIP.To4()
		connectReq = []byte{
			0x05, // Version 5
			0x01, // CONNECT command
			0x00, // Reserved
			0x01, // IPv4 address type
			ip4[0], ip4[1], ip4[2], ip4[3],
			byte(targetPort >> 8), byte(targetPort & 0xFF),
		}
	} else {
		// Domain name
		hostname := config.RawSocketTest.TargetHost
		if len(hostname) > 255 {
			log.Println("Hostname too long for SOCKS5")
			atomic.AddUint64(&proxyErr, 1)
			return false
		}

		connectReq = []byte{
			0x05,              // Version 5
			0x01,              // CONNECT command
			0x00,              // Reserved
			0x03,              // Domain name address type
			byte(len(hostname)), // Domain name length
		}
		connectReq = append(connectReq, []byte(hostname)...)
		connectReq = append(connectReq, byte(targetPort>>8), byte(targetPort&0xFF))
	}

	_, err = conn.Write(connectReq)
	if err != nil {
		atomic.AddUint64(&proxyErr, 1)
		return false
	}

	// Read CONNECT response (variable length)
	connectRespHeader := make([]byte, 4)
	_, err = io.ReadFull(conn, connectRespHeader)
	if err != nil {
		atomic.AddUint64(&proxyErr, 1)
		return false
	}

	if connectRespHeader[1] != 0x00 {
		atomic.AddUint64(&statusCodeErr, 1)
		return false
	}

	// Read rest of response based on address type
	var addrLen int
	switch connectRespHeader[3] {
	case 0x01: // IPv4
		addrLen = 4
	case 0x03: // Domain name
		lenByte := make([]byte, 1)
		io.ReadFull(conn, lenByte)
		addrLen = int(lenByte[0])
	case 0x04: // IPv6
		addrLen = 16
	}
	
	// Read address + port
	io.ReadFull(conn, make([]byte, addrLen+2))

	// Connection established, now send raw bytes
	if config.RawSocketTest.SendHex != "" {
		rawBytes, err := hex.DecodeString(config.RawSocketTest.SendHex)
		if err != nil {
			log.Printf("Invalid hex string: %v\n", err)
			atomic.AddUint64(&proxyErr, 1)
			return false
		}

		_, err = conn.Write(rawBytes)
		if err != nil {
			atomic.AddUint64(&proxyErr, 1)
			return false
		}
	}

	// Check if we expect a response
	if config.RawSocketTest.ExpectResponse {
		responseBuffer := make([]byte, 4096)
		conn.SetReadDeadline(time.Now().Add(timeout))
		
		n, err := conn.Read(responseBuffer)
		if err != nil {
			atomic.AddUint64(&proxyErr, 1)
			if strings.Contains(err.Error(), "timeout") {
				atomic.AddUint64(&timeoutErr, 1)
			}
			return false
		}

		// Check minimum response size
		if config.RawSocketTest.ResponseMinSize > 0 && n < config.RawSocketTest.ResponseMinSize {
			atomic.AddUint64(&statusCodeErr, 1)
			return false
		}
	}

	return true
}

func (p *Proxy) CheckProxySocks5(proxy string) {
	atomic.AddInt64(&p.openHttpThreads, 1)
	defer func() {
		atomic.AddInt64(&p.openHttpThreads, -1)
		atomic.AddUint64(&checked, 1)
		p.processingMu.Lock()
		delete(p.processing, proxy)
		p.processingMu.Unlock()
	}()

	var err error
	var proxyPort = *port
	s := strings.Split(proxy, ":")
	if len(s) > 1 {
		proxyPort, err = strconv.Atoi(strings.TrimSpace(s[1]))
		if err != nil {
			log.Println(err)
			return
		}
	}

	// If raw socket test is enabled, use custom target
	if config.RawSocketTest.Enabled {
		success := p.testRawSocketViaSocks5(s[0], proxyPort)
		if success {
			if config.PrintIps.Enabled {
				go PrintProxy(s[0], proxyPort)
			}
			atomic.AddUint64(&success, 1)
			exporter.Add(fmt.Sprintf("%s:%d", s[0], proxyPort))
		}
		return
	}

	// Original SOCKS5 test with 8.8.8.8
	timeout := time.Second * time.Duration(config.Timeout.Socks5Timeout)
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", s[0], proxyPort), timeout)
	if err != nil {
		atomic.AddUint64(&proxyErr, 1)
		if strings.Contains(err.Error(), "timeout") {
			atomic.AddUint64(&timeoutErr, 1)
		}
		return
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(timeout))

	// SOCKS5 handshake - no authentication
	handshake := []byte{0x05, 0x01, 0x00}
	_, err = conn.Write(handshake)
	if err != nil {
		atomic.AddUint64(&proxyErr, 1)
		return
	}

	// Read response
	handshakeResp := make([]byte, 2)
	_, err = io.ReadFull(conn, handshakeResp)
	if err != nil {
		atomic.AddUint64(&proxyErr, 1)
		return
	}

	if handshakeResp[0] != 0x05 || handshakeResp[1] != 0x00 {
		atomic.AddUint64(&proxyErr, 1)
		return
	}

	// Send CONNECT request to 8.8.8.8:80
	connectReq := []byte{
		0x05,       // Version 5
		0x01,       // CONNECT command
		0x00,       // Reserved
		0x01,       // IPv4 address type
		0x08, 0x08, 0x08, 0x08, // 8.8.8.8
		0x00, 0x50, // Port 80
	}
	_, err = conn.Write(connectReq)
	if err != nil {
		atomic.AddUint64(&proxyErr, 1)
		return
	}

	// Read response
	connectResp := make([]byte, 10)
	_, err = io.ReadFull(conn, connectResp)
	if err != nil {
		atomic.AddUint64(&proxyErr, 1)
		return
	}

	// Check if connection succeeded
	if connectResp[1] != 0x00 {
		atomic.AddUint64(&statusCodeErr, 1)
		return
	}

	// Success
	if config.PrintIps.Enabled {
		go PrintProxy(s[0], proxyPort)
	}
	atomic.AddUint64(&success, 1)
	exporter.Add(fmt.Sprintf("%s:%d", s[0], proxyPort))
}
