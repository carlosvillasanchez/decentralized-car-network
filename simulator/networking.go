package simulator

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
func (simulator *CarNetworkSimulator) listenCars() {
	/*
		simulator.NetworkConnection = createConnection(simulator.NetworkAddress)
		for {

		}
	*/
}

func (simulator *CarNetworkSimulator) listenFront() {
	/*
		simulator.FrontendConnection = createConnection(simulator.FrontendAddress)
		for {

		}
	*/
}

func (simulator *CarNetworkSimulator) listenFrontend() {

}
