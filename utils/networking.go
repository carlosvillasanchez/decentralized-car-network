package utils

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	// "github.com/tormey97/decentralized-car-network/simulator"
)

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
func StringToCarMap(stringFlag string) [9][9]Square {
	var squareType Square
	// make([]int, 0, 5)
	var carGrid [9][9]Square
	// var carMap = Grid
	for i := 0; i < 9; i++ {
		stringSplit := strings.SplitN(stringFlag, ",", 10)
		for j, square := range stringSplit {
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
	x, _ := strconv.ParseUint(PositionArray[0], 10, 32)
	y, _ := strconv.ParseUint(PositionArray[1], 10, 32)
	positionP := Position{
		X: uint32(x),
		Y: uint32(y),
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
func CreatePath(carMap SimulatedMap, startP, endP Position) []Position {
	var dummy []Position
	return dummy
}
func SliceContains(a string, b []string) bool {
	for i := range b {
		if b[i] == a {
			return true
		}
	}
	return false
}
