package simulator

import (
	"net"
	"sync"
)

type Position struct { // TODO will probably be defined elsewhere
	x uint32
	y uint32
}

type Car struct {
	net.UDPAddr
	Position
	Id string
}

type CarsInNetwork struct {
	sync.RWMutex
	Map map[string]Car
}

type Square struct { // One grid/square in the map
	Type string
	Position
}

type SimulatedMap struct { // The entire simulated map
	sync.RWMutex
	Grid [9][9]Square
}

type CarNetworkSimulator struct {
	CarsInNetwork
	SimulatedMap
	net.UDPConn
}
