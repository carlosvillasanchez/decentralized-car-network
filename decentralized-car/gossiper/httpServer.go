/***
* Simulation of a Decentralized Network of Autonomous Cars
* Authors:
* 	- Torstein Meyer
* 	- Fernando Monje
* 	- Carlos Villa
***/
package gossiper

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/tormey97/decentralized-car-network/decentralized-car/messaging"
)

func (peerster *Peerster) handleNewMessage(w http.ResponseWriter, req *http.Request) {
	buffer := make([]byte, 1024)
	n, err := req.Body.Read(buffer)
	if err != nil {
		//fmt.Printf("Could not read message from frontend, reason: %s \n", err)
	}
	decoded := decodeJson(string(buffer[:n]))
	text := decoded["message"].(string)
	destination := decoded["destination"]
	destinationString := ""
	if destination != nil {
		destinationString = destination.(string)
	}
	message := messaging.Message{
		Text:        text,
		Destination: &destinationString,
	}
	peerster.clientReceive(message)
}

func (peerster *Peerster) handleRegisterNode(w http.ResponseWriter, req *http.Request) {
	buffer := make([]byte, 1024)
	n, err := req.Body.Read(buffer)
	if err != nil {
		fmt.Printf("Could not register new node from frontend, reason: %s \n", err)
	}
	peerster.addToKnownPeers(string(buffer[:n]))
}

func decodeJson(jsonString string) (decoded map[string]interface{}) {
	_ = json.Unmarshal([]byte(jsonString), &decoded)
	return
}

func sendValueAsJson(w http.ResponseWriter, req *http.Request, val interface{}) {
	b, err := json.Marshal(val)
	if err != nil {
		fmt.Printf("Could not encode received msgs as json, reason: %s \n", err)
		return
	}
	w.WriteHeader(200)
	_, err = w.Write(b)
	if err != nil {
		fmt.Printf("Could not send messages to frontend, reason: %s \n", err)
		return
	}
}

func (peerster *Peerster) sendMessages(w http.ResponseWriter, req *http.Request) {
	allMessages := struct {
		PrivateMessages map[string][]messaging.PrivateMessage
		RumorMessages   map[string][]messaging.RumorMessage
	}{PrivateMessages: peerster.ReceivedPrivateMessages.Map,
		RumorMessages: peerster.ReceivedMessages.Map}
	sendValueAsJson(w, req, allMessages)
}

func (peerster *Peerster) handleMessage(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		peerster.sendMessages(w, req)
	case http.MethodPost:
		peerster.handleNewMessage(w, req)
	}
}

func (peerster *Peerster) handlePeers(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		sendValueAsJson(w, req, peerster.KnownPeers)
	case http.MethodPost:
		peerster.handleRegisterNode(w, req)
	}
}

func (peerster *Peerster) handleId(w http.ResponseWriter, req *http.Request) {
	sendValueAsJson(w, req, peerster.Name)
}

func (peerster *Peerster) handleHopTable(w http.ResponseWriter, req *http.Request) {
	sendValueAsJson(w, req, peerster.NextHopTable.Map)
}

func (peerster *Peerster) handleFileShare(w http.ResponseWriter, req *http.Request) {
	buffer := make([]byte, 1024)
	n, err := req.Body.Read(buffer)
	if err != nil {
		//fmt.Printf("Could not read message from frontend, reason: %s \n", err)
	}
	peerster.shareFile(string(buffer[:n]))
}

func (peerster *Peerster) handleGetSharedFiles(w http.ResponseWriter, req *http.Request) {
	sendValueAsJson(w, req, peerster.SharedFiles)
}

func (peerster *Peerster) handleRequestFile(w http.ResponseWriter, req *http.Request) {
	buffer := make([]byte, 1024)
	n, err := req.Body.Read(buffer)
	if err != nil {
		//fmt.Printf("Could not read message from frontend, reason: %s \n", err)
	}
	decoded := decodeJson(string(buffer[:n]))
	println(decoded)
	fmt.Println(decoded["fileName"], decoded["metafileHash"], decoded["destination"])
	if decoded["fileName"] == nil || decoded["metafileHash"] == nil || decoded["destination"] == nil {
		return
	}
	fileName := decoded["fileName"].(string)
	metafileHash := decoded["metafileHash"].(string)
	destination := decoded["destination"].(string)
	message := messaging.Message{
		Destination: &destination,
		Request:     metafileHash,
		File:        fileName,
	}
	peerster.clientReceive(message)
}

func (peerster *Peerster) ListenFrontend() {
	http.HandleFunc("/message", peerster.handleMessage)
	http.HandleFunc("/node", peerster.handlePeers)
	http.HandleFunc("/id", peerster.handleId)
	http.HandleFunc("/hop-table", peerster.handleHopTable)
	http.HandleFunc("/share-file", peerster.handleFileShare)
	http.HandleFunc("/get-shared-files", peerster.handleGetSharedFiles)
	http.HandleFunc("/request-file", peerster.handleRequestFile)
	http.Handle("/", http.FileServer(http.Dir("./static")))

	frontendPort := 8080
	success := false
	for !success {
		err := http.ListenAndServe(":"+strconv.Itoa(frontendPort), nil)
		if err == nil {
			success = true
		} else {
			frontendPort++
		}
	}
}
