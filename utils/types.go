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
type ColisionInformation struct {
	NumberColisions int
	IPCar           string
	CoinFlip        int
}
type CarNetworkSimulator struct {
	CarsInNetwork
	SimulatedMap
	net.UDPConn
}
type CarInformation struct {
	Origin   string // Name of the car
	Position Position
	IPCar    string
	Channel  chan bool
}
type CarInfomartionList struct {
	Slice []*CarInformation
	Mutex sync.RWMutex
}
