/***
* Simulation of a Decentralized Network of Autonomous Cars
* Authors:
* 	- Torstein Meyer
* 	- Fernano Monje
* 	- Carlos Villa
***/
package utils

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	// "github.com/tormey97/decentralized-car-network/simulator"
)

const ServerAddress string = "127.0.0.1:5999"
const MaxBufferSize = 65536

func ReadFromConnection(conn net.UDPConn) ([]byte, *net.UDPAddr, error) {
	buffer := make([]byte, MaxBufferSize)
	n, originAddr, err := conn.ReadFromUDP(buffer)
	if err != nil {
		return nil, nil, err
	}
	//fmt.Printf("Amount of bytes read: %s | from: %s \n", n, originAddr.String())

	buffer = buffer[:n]
	return buffer, originAddr, nil
}

func StringAddrToUDPAddr(addr string) net.UDPAddr {
	ipAndPort := strings.Split(addr, ":")
	if len(ipAndPort) < 2 {
		fmt.Printf("Warning: the address %q has the wrong format \n", addr)
		return net.UDPAddr{}
	}
	port, err := strconv.Atoi(ipAndPort[1])
	ip := strings.Split(ipAndPort[0], ".")
	ipByte := []byte{}
	for i := range ip {
		ipInt, err := strconv.Atoi(ip[i])
		if err != nil {
			break
		}
		ipByte = append(ipByte, byte(ipInt))
	}
	if err != nil {
		return net.UDPAddr{}
	}
	return net.UDPAddr{
		IP:   ipByte,
		Port: port,
		Zone: "",
	}
}

// This function takes a string of the map and transforms it to a matrix of Square objects
func StringToCarMap(stringFlag string) [10][10]Square {
	var squareType Square
	// make([]int, 0, 5)
	var carGrid [10][10]Square
	// var carMap = Grid
	for i := 0; i < 10; i++ {
		stringSplit := strings.SplitN(stringFlag, ",", 11)
		if i != 9 {
			stringFlag = stringSplit[10]
		}
		for j, square := range stringSplit[:10] {
			squareType.Type = square
			carGrid[i][j] = squareType
		}
	}
	return carGrid
}

// This function  takes a map as array and returns it in string format
func ArrayStringToString(incomingArray [][]string) string {

	var strs []string

	for _, value := range incomingArray {
		rowString := strings.Join(value, ", ")
		strs = append(strs, rowString)

	}
	finalString := strings.Join(strs, ", ")
	return finalString
}

// This function takes the position in string format and returns it in Position formar
func StringToPosition(posString string) Position {
	PositionArray := strings.Split(posString, ",")
	// startPositionP.X =
	x, _ := strconv.Atoi(PositionArray[0])
	y, _ := strconv.Atoi(PositionArray[1])
	positionP := Position{
		X: x,
		Y: y,
	}
	return positionP
}
func SliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

// This functions return the area of the given position
func AreaPositioner(position Position) int {
	/*switch {
	case position.X < 3 && position.Y < 3:
		fmt.Println("Area 1")
		return 1
	case position.X < 6 && position.Y < 3:
		fmt.Println("Area 2")
		return 2
	case position.X < 9 && position.Y < 3:
		fmt.Println("Area 3")
		return 3
	case position.X < 3 && position.Y < 6:
		fmt.Println("Area 4")
		return 4
	case position.X < 6 && position.Y < 6:
		fmt.Println("Area 5")
		return 5
	case position.X < 9 && position.Y < 6:
		fmt.Println("Area 6")
		return 6
	case position.X < 3 && position.Y < 9:
		fmt.Println("Area 7")
		return 7
	case position.X < 6 && position.Y < 9:
		fmt.Println("Area 8")
		return 8
	case position.X < 9 && position.Y < 9:
		fmt.Println("Area 9")
		return 9
	default:
		fmt.Println("Mayday, America has been discovered")
		return -1
	}*/
	switch {
	case position.X < 5 && position.Y < 5:
		return 1
	case position.X < 10 && position.Y < 5:
		return 2
	case position.X < 5 && position.Y < 10:
		return 3
	case position.X < 10 && position.Y < 10:
		return 4
	default:
		return -1
	}
}
func SliceContains(a string, b []string) bool {
	for i := range b {
		if b[i] == a {
			return true
		}
	}
	return false
}
