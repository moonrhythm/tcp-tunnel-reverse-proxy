package main

import (
	"bytes"
	"flag"
	"io"
	"log"
	"net"
	"time"
)

var (
	proxyAddr    = flag.String("proxy-addr", ":8080", "Reverse Proxy Address")
	registerAddr = flag.String("register-addr", ":8081", "Register Address")
	clientMode   = flag.Bool("client", false, "Start client mode")
)

func main() {
	flag.Parse()

	if *clientMode {
		go startClientService()
	} else {
		go startProxyService()
		go startRegisterService()
	}

	select {}
}

var (
	connList = make(chan net.Conn) // unbuffered channel, only accept connection when there are any waiting client
)

func startProxyService() {
	log.Printf("start proxy service on %s", *proxyAddr)
	lis, err := net.Listen("tcp", *proxyAddr)
	if err != nil {
		log.Fatalf("can not start proxy service; %v", err)
	}
	defer lis.Close()

	h := func(conn net.Conn) {
		var srvConn net.Conn

		for {
			srvConn = <-connList

			// check is srvConn timed out
			_, err := srvConn.Read([]byte{})
			if err == nil {
				break
			}
			log.Printf("connection timed out from %s", srvConn)
		}

		defer conn.Close()
		defer srvConn.Close()

		log.Printf("tunneling %s <=> %s", conn.RemoteAddr(), srvConn.RemoteAddr())
		done := make(chan struct{})
		go func() {
			io.Copy(conn, srvConn)
			done <- struct{}{}
		}()
		go func() {
			io.Copy(srvConn, conn)
			done <- struct{}{}
		}()
		<-done
	}

	for {
		conn, err := lis.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				time.Sleep(5 * time.Millisecond)
				continue
			}
			return
		}
		log.Printf("received proxy connection from %s", conn.RemoteAddr())
		go h(conn)
	}
}

func startRegisterService() {
	log.Printf("start register service on %s", *registerAddr)
	lis, err := net.Listen("tcp", *registerAddr)
	if err != nil {
		log.Fatalf("can not start register service; %v", err)
	}
	defer lis.Close()

	for {
		conn, err := lis.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				time.Sleep(5 * time.Millisecond)
				continue
			}
			return
		}
		log.Printf("received register connection from %s", conn.RemoteAddr())
		connList <- conn
	}
}

func startClientService() {
	log.Printf("start client service")
	log.Printf("proxy address: %s", *proxyAddr)
	log.Printf("register address=%s", *registerAddr)

	h := func() {
		log.Printf("dialing register service...")
		regConn, err := net.Dial("tcp", *registerAddr)
		if err != nil {
			log.Printf("dial error retrying...; err=%v", err)
			time.Sleep(2 * time.Second)
			return
		}
		defer regConn.Close()

		log.Printf("waiting first byte...")
		var buf [1]byte
		_, err = regConn.Read(buf[:])
		if err != nil {
			return
		}

		log.Printf("dialing proxy...")
		proxyConn, err := net.Dial("tcp", *proxyAddr)
		if err != nil {
			return
		}
		defer proxyConn.Close()

		io.Copy(proxyConn, bytes.NewReader(buf[:]))

		log.Printf("tunnel %s <=> %s", regConn.RemoteAddr(), proxyConn.RemoteAddr())

		done := make(chan struct{})
		go func() {
			io.Copy(proxyConn, regConn)
			done <- struct{}{}
		}()
		go func() {
			io.Copy(regConn, proxyConn)
			done <- struct{}{}
		}()
		<-done
	}

	for {
		h()
	}
}
