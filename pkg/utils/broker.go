package utils

import (
	"context"
	"fmt"
)

// Broker code adapted from https://stackoverflow.com/questions/36417199/how-to-broadcast-message-using-channel
// which is licensed under Creative Commons CC BY-SA 4.0.

// Broker implements a simple pub-sub broadcast system
type Broker struct {
	ctx       context.Context
	publishCh chan map[string]string
	subCh     chan chan map[string]string
	unsubCh   chan chan map[string]string
}

// NewBroker allocates a new Broker object
func NewBroker(ctx context.Context) *Broker {
	b := &Broker{
		ctx:       ctx,
		publishCh: make(chan map[string]string),
		subCh:     make(chan chan map[string]string),
		unsubCh:   make(chan chan map[string]string),
	}
	go b.start()
	return b
}

// start starts the broker goroutine
func (b *Broker) start() {
	subs := map[chan map[string]string]struct{}{}
	for {
		select {
		case <-b.ctx.Done():
			return
		case msgCh := <-b.subCh:
			subs[msgCh] = struct{}{}
		case msgCh := <-b.unsubCh:
			delete(subs, msgCh)
		case msg := <-b.publishCh:
			for msgCh := range subs {
				// msgCh is buffered, use non-blocking send to protect the broker:
				select {
				case msgCh <- msg:
				default:
				}
			}
		}
	}
}

// Subscribe registers to receive messages from the broker
func (b *Broker) Subscribe() chan map[string]string {
	if b == nil || b.ctx == nil {
		fmt.Printf("foo\n")
	}
	if b.ctx.Err() == nil {
		msgCh := make(chan map[string]string, 1)
		b.subCh <- msgCh
		return msgCh
	}
	return nil
}

// Unsubscribe de-registers a message receiver
func (b *Broker) Unsubscribe(msgCh chan map[string]string) {
	if b.ctx.Err() == nil {
		b.unsubCh <- msgCh
	}
	close(msgCh)
}

// Publish sends a message to all subscribers
func (b *Broker) Publish(msg map[string]string) {
	if b.ctx.Err() == nil {
		b.publishCh <- msg
	}
}
