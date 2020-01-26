package utils

import (
	"net"
	"sync"
)

const (
	Parking = "parking" // For traces regarding parking spots
	Crash   = "crash"   // For traces regarding the avoidance of crashes
	Police  = "police"  // For traces regarding crash handling
	Other   = "other"   // All rest of traces
)

// Recieve message info if we are entering a spot or accident zone
type ServerMessage struct {
	Type string
}

// Send position or trace
type ServerNodeMessage struct {
	Position *Position
	Trace    *MessageTrace
}

// Traces to the server like, downloading file, accident spoted, colision management etc
type MessageTrace struct {
	Type string
	Text string
}
type Position struct { // TODO will probably be defined elsewhere
	X int
	Y int
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
