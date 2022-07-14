package main

import (
	"context"
	"encoding/json"

	"github.com/libp2p/go-libp2p-core/peer"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// SubscriptionBufSize is the number of incoming messages to buffer for each topic.
const SubscriptionBufSize = 128

// NetworkSubscription represents a subscription to a single PubSub topic. Messages
// can be published to the topic with NetworkSubscription.Publish, and received
// messages are pushed to the Messages channel.
type NetworkSubscription struct {
	// Messages is a channel of messages received from other peers in the topic
	Messages chan *TickMessage

	ctx   context.Context
	ps    *pubsub.PubSub
	topic *pubsub.Topic
	sub   *pubsub.Subscription

	topicName string
	selfID    peer.ID
	peerName  string
}

// JoinNetwork tries to subscribe to the specified PubSub topic, returning
// a Subscription on success.
func JoinNetwork(ctx context.Context, ps *pubsub.PubSub, selfID peer.ID, peerName string, topicName string) (*NetworkSubscription, error) {
	// join the pubsub topic
	topic, err := ps.Join(topicName)
	if err != nil {
		return nil, err
	}

	// and subscribe to it
	sub, err := topic.Subscribe()
	if err != nil {
		return nil, err
	}

	nsub := &NetworkSubscription{
		ctx:       ctx,
		ps:        ps,
		topic:     topic,
		sub:       sub,
		selfID:    selfID,
		peerName:  peerName,
		topicName: topicName,
		Messages:  make(chan *TickMessage, SubscriptionBufSize),
	}

	// start reading messages from the subscription in a loop
	go nsub.readLoop()
	return nsub, nil
}

// Publish sends a message to the pubsub topic.
func (nsub *NetworkSubscription) Publish(message string) error {
	tm := TickMessage{
		Message:        message,
		SenderID:       nsub.selfID.Pretty(),
		SenderPeerName: nsub.peerName,
	}
	msgBytes, err := json.Marshal(tm)
	if err != nil {
		return err
	}
	return nsub.topic.Publish(nsub.ctx, msgBytes)
}

func (nsub *NetworkSubscription) ListPeers() []peer.ID {
	return nsub.ps.ListPeers(nsub.topicName)
}

// readLoop pulls messages from the pubsub topic and pushes them onto the Messages channel.
func (nsub *NetworkSubscription) readLoop() {
	for {
		msg, err := nsub.sub.Next(nsub.ctx)
		if err != nil {
			close(nsub.Messages)
			return
		}
		// only forward messages delivered by others
		if msg.ReceivedFrom == nsub.selfID {
			continue
		}
		tm := new(TickMessage)
		err = json.Unmarshal(msg.Data, tm)
		if err != nil {
			continue
		}
		// send valid messages onto the Messages channel
		nsub.Messages <- tm
	}
}
