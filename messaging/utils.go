package messaging

import (
	"fmt"
	"net"
	"strconv"
	"strings"
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

func SliceContains(a string, b []string) bool {
	for i := range b {
		if b[i] == a {
			return true
		}
	}
	return false
}
