package gossiper

import (
	"container/heap"
	"fmt"
	"github.com/tormey97/decentralized-car-network/simulator"
)

/*
This file contains all the functionality allowing for navigation through the network.
what functions do we need?
createPath(*peerster) (currentPos, targetPos, obstructions) -> [](int, int)

*/

type pathfindingNode struct {
	simulator.Position
	cost        int
	predecessor simulator.Position
}

// Returns the positions of the neighbors of a pathfindingNode
func getNeighbors(node pathfindingNode, width, height int) []simulator.Position {
	// We convert to a signed int because we use values less than 0 to see if the node is on the edge of the map
	type signedPosition struct {
		X int
		Y int
	}
	leftNeighbor := signedPosition{
		X: int(node.Position.X) - 1,
		Y: int(node.Position.Y),
	}
	rightNeighbor := signedPosition{
		X: int(node.Position.X) + 1,
		Y: int(node.Position.Y),
	}
	belowNeighbor := signedPosition{
		X: int(node.Position.X),
		Y: int(node.Position.Y) - 1,
	}
	aboveNeighbor := signedPosition{
		X: int(node.Position.X),
		Y: int(node.Position.Y) + 1,
	}

	var neighbors []simulator.Position
	for _, neighbor := range []signedPosition{leftNeighbor, rightNeighbor, belowNeighbor, aboveNeighbor} {
		if neighbor.X >= 0 && neighbor.X < width && neighbor.Y >= 0 && neighbor.Y < height {
			neighbors = append(neighbors, simulator.Position{
				X: uint32(neighbor.X),
				Y: uint32(neighbor.Y),
			})
		}
	}
	return neighbors
}

type nodePriorityQueue []pathfindingNode

func (h nodePriorityQueue) Len() int           { return len(h) }
func (h nodePriorityQueue) Less(i, j int) bool { return h[i].cost < h[j].cost }
func (h nodePriorityQueue) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h nodePriorityQueue) Contains(x pathfindingNode) bool {
	for i := range h {
		if h[i].X == x.X && h[i].Y == x.Y {
			return true
		}
	}
	return false
}
func (h *nodePriorityQueue) Push(x interface{}) {
	*h = append(*h, x.(pathfindingNode))
}

func (h *nodePriorityQueue) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

func toPathfindingNode(pos simulator.Position) pathfindingNode {
	return pathfindingNode{
		Position: pos,
		cost:     0,
	}
}

// need a struct for the detected cost for each checked node
// [][]x, y, cost
func CreatePath(
	simulatedMap *simulator.SimulatedMap,
	startPos, endPos simulator.Position,
	obstructions []simulator.Position) []simulator.Position {
	simulatedMap.RLock()
	distances := [9][9]pathfindingNode{}
	openSet := nodePriorityQueue{toPathfindingNode(startPos)}
	closedSet := nodePriorityQueue{}
	pathFound := false
	width, height := len(simulatedMap.Grid), len(simulatedMap.Grid[0])
	for openSet.Len() > 0 && !pathFound {
		currentNode := heap.Pop(&openSet).(pathfindingNode)
		closedSet.Push(currentNode)
		neighbors := getNeighbors(currentNode, width, height)
		for _, neighbor := range neighbors {
			nodeType := simulatedMap.Grid[neighbor.X][neighbor.Y].Type
			neighborNode := distances[neighbor.X][neighbor.Y]
			if currentNode.cost+1 < neighborNode.cost {
				neighborNode.cost = currentNode.cost + 1
				neighborNode.predecessor = currentNode.Position
			}
			if !openSet.Contains(currentNode) && !closedSet.Contains(currentNode) {
				heap.Push(&openSet, currentNode)
			}
			if neighborNode.Position == endPos {
				pathFound = true
			}
		}
	}
	if pathFound {

	}
	return nil
}

func main() {
	fmt.Printf("Eh")
}
