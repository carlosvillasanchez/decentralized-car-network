package main

import (
	"flag"
	"fmt"
	//"github.com/TorsteinMeyer/Peerster/messaging"
)

func main() {
	UIPort := flag.String("UIPort", "8080", "port for the UI client (default '8080'")
	msg := flag.String("msg", "Default message", "message to be sent")
	fmt.Printf("UIPort: %q, msg: %q, test", UIPort, msg)
}
