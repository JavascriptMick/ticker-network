package main

import (
	"time"
)

// TickMessage gets converted to/from JSON and sent in the body of pubsub messages.
type TickMessage struct {
	Message        string
	SenderID       string
	SenderPeerName string
}

// message to display, currently the same as the network message but will diverge
type DisplayMessage struct {
	Message        string
	SenderID       string
	SenderPeerName string
}

// TickMessageBuffSize is the number of ticks to buffer for each topic.
const TickMessageBuffSize = 128
const MillisecondsPerSubTick = 10
const SubTicksPerTick = 250 // 250 * 10 = 2500 ms (2.5 Seconds) between ticks
const BumpClock float64 = 0.08

type Ticker struct {
	// Channel to send messages to the UI for display
	DisplayMessages chan *DisplayMessage

	// Subscription to the network
	nsub *NetworkSubscription
}

func (ticker *Ticker) tickLoop() {
	for {
		// Each tick is broken down into several hundred 'sub ticks'
		// in between sub ticks, we check the messages channel to see if peers have ticked
		// if so, we advance our clock so we tick faster next time.
		clock := 0
		for {
			clock++
			if clock >= SubTicksPerTick {
				break // time to tick
			}
			time.Sleep(MillisecondsPerSubTick * time.Millisecond)

			select {
			case tm, ok := <-ticker.nsub.Messages:
				if ok {
					// tell the UI that An external node ticked
					dm, _ := DisplayMessageFromTickMessage(tm)
					dm.Message += "(external)"
					ticker.DisplayMessages <- dm

					// move clock forward
					clock += int(float64(clock) * BumpClock)
				} else {
					// fmt.Println("Channel closed!")
				}
			default:
				// fmt.Println("No value ready, moving on.")
			}
		}
		// tell the network that I ticked
		ticker.nsub.Publish("tick")

		// tell the UI that I ticked
		dm := new(DisplayMessage)
		dm.SenderID = ticker.nsub.selfID.Pretty()
		dm.Message = "Tick (Self)"
		dm.SenderPeerName = ticker.nsub.peerName

		ticker.DisplayMessages <- dm
	}
}

func StartTicking(nsub *NetworkSubscription) (*Ticker, error) {
	ticker := &Ticker{
		DisplayMessages: make(chan *DisplayMessage, TickMessageBuffSize),
		nsub:            nsub,
	}

	// start reading messages from the subscription in a loop
	go ticker.tickLoop()
	return ticker, nil
}

func DisplayMessageFromTickMessage(tm *TickMessage) (*DisplayMessage, error) {
	dm := new(DisplayMessage)
	dm.Message = tm.Message
	dm.SenderID = tm.SenderID
	dm.SenderPeerName = tm.SenderPeerName
	return dm, nil
}
