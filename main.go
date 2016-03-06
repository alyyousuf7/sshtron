package main

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"net"
	"net/http"
	"os"
)

const (
	sshPortEnv  = "SSH_PORT"
	httpPortEnv = "PORT"

	defaultSshPort  = "2022"
	defaultHttpPort = "3000"
)

func handler(conn net.Conn, gm *GameManager, config *ssh.ServerConfig) {
	// Before use, a handshake must be performed on the incoming
	// net.Conn.
	_, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		panic("failed to handshake")
	}
	// The incoming Request channel must be serviced.
	go ssh.DiscardRequests(reqs)

	// Service the incoming Channel channel.
	for newChannel := range chans {
		// Channels have a type, depending on the application level
		// protocol intended. In the case of a shell, the type is
		// "session" and ServerShell may be used to present a simple
		// terminal interface.
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}
		channel, requests, err := newChannel.Accept()
		if err != nil {
			panic("could not accept channel.")
		}

		// Reject all out of band requests accept for the unix defaults, pty-req and
		// shell.
		go func(in <-chan *ssh.Request) {
			for req := range in {
				fmt.Println("New request:", req.Type)
				switch req.Type {
				case "pty-req":
					req.Reply(true, nil)
					continue
				case "shell":
					req.Reply(true, nil)
					continue
				}
				req.Reply(false, nil)
			}
		}(requests)

		fmt.Println("Received new connection")

		gm.HandleChannel <- channel
	}
}

func port(env, def string) string {
	port := os.Getenv(env)
	if port == "" {
		port = def
	}

	return fmt.Sprintf(":%s", port)
}

func main() {
	sshPort := port(sshPortEnv, defaultSshPort)
	httpPort := port(httpPortEnv, defaultHttpPort)

	// Everyone can login!
	config := &ssh.ServerConfig{
		NoClientAuth: true,
	}

	privateBytes, err := ioutil.ReadFile("id_rsa")
	if err != nil {
		panic("Failed to load private key")
	}

	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		panic("Failed to parse private key")
	}

	config.AddHostKey(private)

	// Create the GameManager
	gm := NewGameManager()
	go gm.Run()

	fmt.Printf(
		"Listening on port %s for SSH and port %s for HTTP...\n",
		sshPort,
		httpPort,
	)

	go func() {
		panic(http.ListenAndServe(httpPort, http.FileServer(http.Dir("./static/"))))
	}()

	// Once a ServerConfig has been configured, connections can be
	// accepted.
	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0%s", sshPort))
	if err != nil {
		panic("failed to listen for connection")
	}

	for {
		nConn, err := listener.Accept()
		if err != nil {
			panic("failed to accept incoming connection")
		}

		go handler(nConn, gm, config)
	}
}
