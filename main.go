package main

import (
	"fmt"
	"github.com/tormey97/decentralized-car-network/decentralized-car/gossiper"
	"github.com/tormey97/decentralized-car-network/utils"
	"sync"
)

/*
This is the centralized server that simulates the world the cars find themselves in.
Handles:
Creation of the map
Management of map updates, these updates will be transmitted to the cars that can "see" the updates via the network
Manages the zones
Creation of cars
Starting the simulation
*/

/*
Flow of program: Client creates map -> specifies amount of cars -> sends command to backend to start program ->
backend sets the map, generates the cars (starts the car programs with initial positions), cars immediately start driving
*/
func main() {
	//TODO remove below, just for testing
	simmap := utils.SimulatedMap{
		RWMutex: sync.RWMutex{},
		Grid:    [9][9]utils.Square{},
	}
	startPos := utils.Position{
		X: 2,
		Y: 0,
	}
	endPos := utils.Position{
		X: 6,
		Y: 5,
	}
	fmt.Println(gossiper.CreatePath(&simmap, startPos, endPos, []utils.Position{{2, 1}, {3, 1}, {4, 1}, {2, 1}}))

}
