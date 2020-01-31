/***
* Simulation of a Decentralized Network of Autonomous Cars
* Authors:
* 	- Torstein Meyer
* 	- Fernano Monje
* 	- Carlos Villa
***/
package simulator

/*
	The code of the simulator is the "main.go" file and not this file. This is a utils for the cars
*/

import (
	"log"
	"net"

	"github.com/tormey97/decentralized-car-network/utils"
)

func createConnection(addr string) (*net.UDPConn, error) {
	udpAddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		return nil, err
	}
	udpConn, err := net.ListenUDP("udp4", udpAddr)
	if err != nil {
		return nil, err
	}
	return udpConn, nil
}

// Reads from
func readConnection(conn *net.UDPConn) ([]byte, *net.UDPAddr, error) {
	buffer, originAddr, err := utils.ReadFromConnection(*conn)
	if err != nil {
		log.Printf("Could not read from connection, origin: %s, reason: %s \n", originAddr, err)
		return nil, nil, err
	}
	return buffer, originAddr, nil
}
