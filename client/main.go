package main

import (
	"flag"
	"log"
	"net"
	//"github.com/TorsteinMeyer/Peerster/messaging"
)

type PeersterClient struct {
	UIPort string
	msg    string
}

func (client PeersterClient) connect() (net.Conn, error) {
	return net.Dial("udp", "127.0.0.1:"+client.UIPort)
}

func createClient() PeersterClient {
	UIPort := flag.String("UIPort", "10000", "port for the UI client (default '8080'")
	msg := flag.String("msg", "Default message", "message to be sent")
	flag.Parse()
	return PeersterClient{
		UIPort: *UIPort,
		msg:    *msg,
	}
}

func main() {
	client := createClient()
	conn, err := client.connect()
	if err != nil {
		log.Fatalf("Fatal error: PeersterClient was unable to connect. %s", err)
	}
	conn.Write([]byte(client.msg))
}
