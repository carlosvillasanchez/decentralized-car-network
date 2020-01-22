package utils

import (
	"net"
	"sync"
)

type Position struct { // TODO will probably be defined elsewhere
	X uint32
	Y uint32
}

type CarPosition struct { // This is the position a car can have
	sync.RWMutex
	Position
}

type Car struct {
	net.UDPAddr
	CarPosition
	Id string
}

type CarsInNetwork struct {
	sync.RWMutex
	Map map[string]Car
}

type Square struct { // One grid/square in the map
	Type string // Type of the square, parking spot, accident, normal etc
}

type SimulatedMap struct { // The entire simulated map
	sync.RWMutex
	Grid [9][9]Square // Matrix representing the whole map
}

type CarNetworkSimulator struct {
	CarsInNetwork
	SimulatedMap
	net.UDPConn
}
