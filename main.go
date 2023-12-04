package main

import (
	"crypto/tls"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/elazarl/goproxy"
	utls "github.com/refraction-networking/utls"
)

var CERT utls.Certificate
var CERT_TLS tls.Certificate

func main() {
	proxy := goproxy.NewProxyHttpServer()

	CERT, err := utls.LoadX509KeyPair("server-cert.pem", "server-key.pem")
	if err != nil {
		log.Fatalf("utls.LoadX509KeyPair() error: %s", err)
	}

	httpProxy, err := net.Listen("tcp", ":8443")
	if err != nil {
		log.Fatalf("utls.Listen() error: %s", err)
	}

	// Intercept HTTP requests
	proxy.OnRequest().HijackConnect(func(req *http.Request, client net.Conn, ctx *goproxy.ProxyCtx) {
		// make tls connection to the destination with utls
		serverName := req.URL.Hostname()
		log.Printf("serverName: %s", serverName)

		client.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))

		uServer := utls.Server(client, &utls.Config{ServerName: serverName, InsecureSkipVerify: true, Certificates: []utls.Certificate{CERT}})
		if err := uServer.Handshake(); err != nil {
			log.Printf("localServer.Handshake() error: %s", err)
			return
		}

		//Dial 8443 and pipe
		dialer, err := net.Dial("tcp", ":8443")
		if err != nil {
			log.Printf("net.Dial() error: %s", err)
			return
		}

		// Pipe both ways
		go func() {
			defer dialer.Close()
			io.Copy(dialer, uServer)
		}()
		go func() {
			defer uServer.Close()
			io.Copy(uServer, dialer)
		}()

	})

	go func() {
		for {
			conn, err := httpProxy.Accept()
			if err != nil {
				log.Printf("tlsProxy.Accept() error: %s", err)
				return
			}
			go handleLocalTLS(conn)
		}
	}()
	log.Fatal(http.ListenAndServe(":8081", proxy))
}

func handleLocalTLS(conn net.Conn) {
	// Read buffer
	buffer := make([]byte, 0)
	for {
		tmp := make([]byte, 1024)
		n, err := conn.Read(tmp)
		if err != nil {
			log.Printf("conn.Read() error: %s", err)
			return
		}
		buffer = append(buffer, tmp[:n]...)
		if n < 1024 {
			break
		}
	}

	host := ""
	for _, line := range strings.Split(string(buffer), "\r\n") {
		if strings.HasPrefix(line, "Host: ") {
			host = strings.TrimPrefix(line, "Host: ")
			break
		}
	}

	rConn, err := net.Dial("tcp", host+":https")
	if err != nil {
		log.Printf("net.Dial() error: %s", err)
		return
	}

	//file writer io
	os.Create("/home/andreas/Desktop/gokeylog")
	f, err := os.OpenFile("/home/andreas/Desktop/gokeylog", os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	uClient := utls.UClient(rConn, &utls.Config{ServerName: host, InsecureSkipVerify: true, KeyLogWriter: f, Certificates: []utls.Certificate{CERT}}, utls.HelloChrome_Auto)
	if err := uClient.Handshake(); err != nil {
		log.Printf("uClient.Handshake() error: %s", err)
		return
	}

	log.Println("uClient.Handshake() success")

	//Send as request
	uClient.Write(buffer)
	recvBuffer := make([]byte, 0)
	for {
		tmp := make([]byte, 2048)
		n, err := uClient.Read(tmp)
		if err != nil {
			log.Printf("uClient.Read() error: %s", err)
			return
		}

		// conn.Write(tmp[:n])
		recvBuffer = append(recvBuffer, tmp[:n]...)
		if n < 2048 {
			break
		}
	}

	//hex to string

	//If is redirect then follow
	// log.Printf("buffer: %x", string(recvBuffer))

	defer conn.Close()
	defer rConn.Close()

	log.Println("conn.Write() success")
}