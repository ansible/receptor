package utils

import "context"

// Broker code adapted from https://stackoverflow.com/questions/36417199/how-to-broadcast-message-using-channel
// which is licensed under Creative Commons CC BY-SA 4.0.

// Broker implements a simple pub-sub broadcast system
type Broker struct {
	ctx       context.Context
	publishCh chan interface{}
	subCh     chan chan interface{}
	unsubCh   chan chan interface{}
}

// NewBroker allocates a new Broker object
func NewBroker(ctx context.Context) *Broker {
	b := &Broker{
		ctx:       ctx,
		publishCh: make(chan interface{}),
		subCh:     make(chan chan interface{}),
		unsubCh:   make(chan chan interface{}),
	}
	go b.start()
	return b
}

// start starts the broker goroutine
func (b *Broker) start() {
	subs := map[chan interface{}]struct{}{}
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
func (b *Broker) Subscribe() chan interface{} {
	msgCh := make(chan interface{}, 1)
	b.subCh <- msgCh
	return msgCh
}

// Unsubscribe de-registers a message receiver
func (b *Broker) Unsubscribe(msgCh chan interface{}) {
	b.unsubCh <- msgCh
	close(msgCh)
}

// Publish sends a message to all subscribers
func (b *Broker) Publish(msg interface{}) {
	b.publishCh <- msg
}
