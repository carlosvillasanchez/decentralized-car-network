package main

import (
	"flag"
	"fmt"
	"github.com/dedis/protobuf"
	"github.com/tormey97/Peerster/messaging"
	"log"
	"net"
	"time"
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

	go func() {
		conn, _ := net.ListenPacket("udp", "127.0.0.1:4521")
		buffer := make([]byte, 1024)
		conn.ReadFrom(buffer)
		b := messaging.GossipPacket{}
		fmt.Println("we get here?")
		err := protobuf.Decode(buffer, &b)
		if err != nil {
			fmt.Println(b, buffer, err, "????")
		}
		fmt.Println(b.Simple.Contents, b.Simple.OriginalName, b.Simple.RelayPeerAddr, "wait you can actually read?")
	}()

	time.Sleep(1000 * time.Millisecond)
	conn1, _ := net.Dial("udp", "127.0.0.1:4521")
	testMessage := messaging.SimpleMessage{
		Contents:      "whats GOING ON BOIS",
		OriginalName:  "name",
		RelayPeerAddr: "what",
	}
	testPacket := messaging.GossipPacket{Simple: &testMessage}

	result, err := protobuf.Encode(&testPacket)
	fmt.Println(result, "WHAT")
	_, err = conn1.Write(result)
	if err != nil {
		fmt.Println(err)
	}

	client := createClient()
	conn, err := client.connect()
	if err != nil {
		log.Fatalf("Fatal error: PeersterClient was unable to connect. %s", err)
	}
	_, err = conn.Write([]byte(client.msg))
	fmt.Println(err)
}
