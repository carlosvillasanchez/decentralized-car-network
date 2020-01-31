/***
* Simulation of a Decentralized Network of Autonomous Cars
* Authors:
* 	- Torstein Meyer
* 	- Fernando Monje
* 	- Carlos Villa
***/
package gossiper

import "github.com/tormey97/decentralized-car-network/decentralized-car/messaging"
//import "fmt"

/*
This module is responsible for subscribing to Newsgroups. Need a function to subscribe, unsubscribe.
Need to add to peerster a list of subscriptions.
Need to add to rumormessages a newsgroup field
Then, add a function to filter rumormessages based on the field
*/

// subscribes to a new newsgroup
func (peerster *Peerster) SubscribeToNewsgroup(newsgroup string) {
	for _, _newsgroup := range peerster.Newsgroups {
		if _newsgroup == newsgroup {
			return
		}
	}
	peerster.Newsgroups = append(peerster.Newsgroups, newsgroup)
}

// unsubscribes from a newsgroup
func (peerster *Peerster) UnsubscribeFromNewsgroup(newsgroup string) {
	for i := range peerster.Newsgroups {
		//fmt.Println("Number", i)
		if peerster.Newsgroups[i] == newsgroup {
			peerster.Newsgroups = append(peerster.Newsgroups[:i], peerster.Newsgroups[i+1:]...)
			break
		}
	}
}

// returns true if the peerster is subscribed to the message's newsgroup (or if it has no newsgroup), false otherwise
func (peerster *Peerster) FilterMessageByNewsgroup(message messaging.RumorMessage) bool {
	if message.Newsgroup != "" {
		for _, newsgroup := range peerster.Newsgroups {
			if newsgroup == message.Newsgroup {
				return true
			}
		}
		return false
	}
	return true
}
