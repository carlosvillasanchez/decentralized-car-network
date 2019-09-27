package main

import (
	"flag"
	"fmt"
	"log"
	"strings"
	"net"
	//"github.com/TorsteinMeyer/Peerster/messaging"
)

type Peerster struct {
	uiPort string
	gossipAddr string
	knownPeers []string
	name string
	simple bool
}

func createPeerster() Peerster{
	uiPort := flag.String("UIPort", "8080", "the port the client uses to communicate with peerster")
	gossipAddr := flag.String("gossipAddr", "127.0.0.1:5000", "the address of the peerster")
	name := flag.String("name", "nodeA", "the name of the node")
	peers := flag.String("peers", "", "known peers")
	simple := flag.Bool("simple", true, "simple mode")
	flag.Parse()
	return Peerster{
		uiPort:     *uiPort,
		gossipAddr: *gossipAddr,
		knownPeers: strings.Split(*peers, ","),
		name:       *name,
		simple:     *simple,
	}
}

func (peerster *Peerster) listen() {
	l, err := net.Listen("udp", fmt.Sprintf(":%s", peerster.uiPort))
	if err != nil {
		log.Fatalf("Error: could not listen: %s", err);
	}
	for {
		conn, err := l.Accept();
		if err != nil {
			log.Fatalf("Error: l.Accept() failed: %s", err)
		}
		l, err := conn.Read([]byte{})
		if err != nil {
			log.Fatalf("Error: conn.Read() failed: %s", err)
		}
		println(l)
	}
}

func main() {
	peerster := createPeerster()
	peerster.listen()
	fmt.Printf("uiPort: %q, gossipAddr: %q", peerster.uiPort, peerster.gossipAddr)
}
