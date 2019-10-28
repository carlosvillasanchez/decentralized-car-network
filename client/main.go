package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"github.com/dedis/protobuf"
	"github.com/tormey97/Peerster/messaging"
	"log"
	"net"
	//"github.com/TorsteinMeyer/Peerster/messaging"
)

type PeersterClient struct {
	UIPort  string
	msg     string
	dest    string
	file    string
	request string
}

func (client PeersterClient) connect() (net.Conn, error) {
	return net.Dial("udp4", "127.0.0.1:"+client.UIPort)
}

func createClient() PeersterClient {
	UIPort := flag.String("UIPort", "8080", "port for the UI client (default '8080'")
	msg := flag.String("msg", "Default message", "message to be sent")
	dest := flag.String("dest", "", "destination for the private message")
	file := flag.String("file", "", "file to share")
	request := flag.String("request", "", "metafile hash of requested file")
	flag.Parse()
	return PeersterClient{
		UIPort:  *UIPort,
		msg:     *msg,
		dest:    *dest,
		file:    *file,
		request: *request,
	}
}

func main() {
	val, err := hex.DecodeString("977132b205d9b89b4193d511455e846c9b03135c9ade14d763c1b7394e57ad42")
	fmt.Println(val, "????")
	client := createClient()
	conn, err := client.connect()
	if err != nil {
		log.Fatalf("Fatal error: PeersterClient was unable to connect. %s", err)
	}
	msg := messaging.Message{
		Text:        client.msg,
		Destination: &client.dest,
		File:        client.file,
		Request:     client.request,
	}
	encoded, err := protobuf.Encode(&msg)
	if err != nil {
		log.Fatalf("Fatal error: Could not send message to peerster. %s, \n", err)
	}
	n, err := conn.Write(encoded)
	fmt.Println(err, n)
}
