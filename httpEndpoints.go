package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	//"math/rand"
	"encoding/json"
	//"net"
	//"github.com/dedis/protobuf"
	//"sync"
)



/***
* POST /setup
*
* HTTP Endpoint for seting up the map and the car and starting the simulation
***/
func (centralServer *CentralServer) setupAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		if err := r.ParseForm(); err != nil {
            fmt.Fprintf(w, "ParseForm() err: %v", err)
            return
		}
		cars := r.FormValue("cars")
		buildings := r.FormValue("buildings")
		carcrashes := r.FormValue("carcrashes")
		parkingspots := r.FormValue("parkingspots")
		fmt.Println("SETTING UP")
		fmt.Println("Car", cars)
		fmt.Println("Buildings", buildings)
		fmt.Println("Carcrash", carcrashes)
		fmt.Println("ParkingSpot", parkingspots)
		centralServer.setupCentralServer(cars, buildings, carcrashes, parkingspots)
		w.Write([]byte("Everything ok.\n"))
		go centralServer.startProtocol()
	}	
}

/***
* POST /addCar
*
* HTTP Endpoint for adding a new car during the simulation
***/
func (centralServer *CentralServer) addCarAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		if err := r.ParseForm(); err != nil {
            fmt.Fprintf(w, "ParseForm() err: %v", err)
            return
		}
		//[TODO: Check there is no car there]
		car := r.FormValue("car")
		fmt.Println("Adding new car:", car)
		carSplited := strings.Split(car, ",")
		x, _ := strconv.Atoi(carSplited[3])
		y, _ := strconv.Atoi(carSplited[4])
		destinationX, _ := strconv.Atoi(carSplited[5])
		destinationY, _ := strconv.Atoi(carSplited[6])
		newCar := Car{
			Id: carSplited[0],
			IP: carSplited[1],
			Port: carSplited[2],
			X: x,
			Y: y,
			DestinationX: destinationX,
			DestinationY: destinationY,
		}
		centralServer.carsMutex.Lock()
		centralServer.Cars[carSplited[1] + ":" + carSplited[2]] = newCar
		centralServer.carsMutex.Unlock()
		centralServer.printMap()
		w.Write([]byte("Everything ok.\n"))
	}
}

/***
* POST /addParkingSpot
*
* HTTP Endpoint for adding a new parking spot during the simulation
***/
func (centralServer *CentralServer) addParkingSpotAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		if err := r.ParseForm(); err != nil {
            fmt.Fprintf(w, "ParseForm() err: %v", err)
            return
		}
		//[TODO: Check there it can be there]
		parkingSpot := r.FormValue("parkingSpot")
		fmt.Println("Adding new parking spot:", parkingSpot)
		parkingSpotSplitted := strings.Split(parkingSpot, ",")
		x, _ := strconv.Atoi(parkingSpotSplitted[1])
		y, _ := strconv.Atoi(parkingSpotSplitted[2])
		newParkingSpot := ParkingSpot{
			Id: parkingSpotSplitted[0],
			X: x,
			Y: y,
		}
		centralServer.ParkingSpots = append(centralServer.ParkingSpots, newParkingSpot)
		centralServer.mapMutex.Lock()
		centralServer.Map[x][y] = "p"
		centralServer.mapMutex.Unlock()
		centralServer.printMap()
		w.Write([]byte("Everything ok.\n"))
	}
}

/***
* POST /addCarCrash
*
* HTTP Endpoint for adding a new car crash during the simulation
***/
func (centralServer *CentralServer) addCarCrashAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		if err := r.ParseForm(); err != nil {
            fmt.Fprintf(w, "ParseForm() err: %v", err)
            return
		}
		//[TODO: Check there it can be there]
		carCrash := r.FormValue("carCrash")
		fmt.Println("Adding new car crash:", carCrash)
		carCrashSplitted := strings.Split(carCrash, ",")
		x, _ := strconv.Atoi(carCrashSplitted[1])
		y, _ := strconv.Atoi(carCrashSplitted[2])
		newCarCrash := CarCrash{
			Id: carCrashSplitted[0],
			X: x,
			Y: y,
		}
		centralServer.CarCrashes = append(centralServer.CarCrashes, newCarCrash)
		centralServer.mapMutex.Lock()
		centralServer.Map[x][y] = "cc"
		centralServer.mapMutex.Unlock()
		centralServer.printMap()
		w.Write([]byte("Everything ok.\n"))
	}
}

type UpdateUI struct {
	Pos map[string][]int
	Messages map[string][]MessageTrace
}

/***
* GET /update
*
* HTTP Endpoint for updating the info in the UI
***/
func (centralServer *CentralServer) updateAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		toSendPos := make(map[string][]int)
		toSendMessages := make(map[string][]MessageTrace)
		//[TODO: Change this for a read lock, and remove the test code.]
		centralServer.carsMutex.Lock()
		for k, v := range centralServer.Cars {
			toSendPos[v.Id] = []int{v.X, v.Y, v.DestinationX, v.DestinationY}
			toSendMessages[v.Id] = v.Messages
			v.Messages = []MessageTrace{}
			centralServer.Cars[k] = v
		}
		centralServer.carsMutex.Unlock()
		toSend := UpdateUI{
			Pos: toSendPos,
			Messages: toSendMessages,
		}
		js, err := json.Marshal(toSend)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(js)
	}
}

/***
* POST /stop
*
* HTTP Endpoint for seting up the map and the car and starting the simulation
***/
func (centralServer *CentralServer) stopAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		// [TODO: STOP THE NETWORK]
		// [TODO: CLEAN THE CENTRAL SERVER ATRIBUTES]
		fmt.Println("STOPING!")
		w.Write([]byte("Everything ok.\n"))
	}	
}