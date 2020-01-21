package decentralized_car_network

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
	/*simmap := simulator.SimulatedMap{
		RWMutex: sync.RWMutex{},
		Grid:    [9][9]simulator.Square{},
	}*/
}
