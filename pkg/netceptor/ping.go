package netceptor

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// NetcForPing should include all methods of Netceptor needed by the Ping function.
type NetcForPing interface {
	ListenPacket(service string) (PacketConner, error)
	NewAddr(target string, service string) Addr
	NodeID() string
	Context() context.Context
}

// Ping calls CreatePing to sends a single test packet and waits for a reply or error.
func (s *Netceptor) Ping(ctx context.Context, target string, hopsToLive byte) (time.Duration, string, error) {
	return CreatePing(ctx, s, target, hopsToLive)
}

// CreatePing creates Ping by sending a single test packet and waits for a replay or error.
func CreatePing(ctx context.Context, s NetcForPing, target string, hopsToLive byte) (time.Duration, string, error) {
	pc, err := s.ListenPacket("")
	if err != nil {
		return 0, "", err
	}
	ctxPing, ctxCancel := context.WithCancel(ctx)
	defer func() {
		ctxCancel()
		_ = pc.Close()
	}()
	pc.SetHopsToLive(hopsToLive)
	doneChan := make(chan struct{})
	unrCh := pc.SubscribeUnreachable(doneChan)
	defer close(doneChan)
	type errorResult struct {
		err      error
		fromNode string
	}
	errorChan := make(chan errorResult)
	go func() {
		for msg := range unrCh {
			errorChan <- errorResult{
				err:      fmt.Errorf(msg.Problem),
				fromNode: msg.ReceivedFromNode,
			}
		}
	}()
	startTime := time.Now()
	replyChan := make(chan string)
	go func() {
		buf := make([]byte, 8)
		_, addr, err := pc.ReadFrom(buf)
		fromNode := ""
		if addr != nil {
			fromNode = addr.String()
			fromNode = strings.TrimSuffix(fromNode, ":ping")
		}
		if err == nil {
			select {
			case replyChan <- fromNode:
			case <-ctxPing.Done():
			case <-s.Context().Done():
			}
		} else {
			select {
			case errorChan <- errorResult{
				err:      err,
				fromNode: fromNode,
			}:
			case <-ctx.Done():
			case <-s.Context().Done():
			}
		}
	}()
	_, err = pc.WriteTo([]byte{}, s.NewAddr(target, "ping"))
	if err != nil {
		return time.Since(startTime), s.NodeID(), err
	}
	select {
	case errRes := <-errorChan:
		return time.Since(startTime), errRes.fromNode, errRes.err
	case remote := <-replyChan:
		return time.Since(startTime), remote, nil
	case <-time.After(10 * time.Second):
		return time.Since(startTime), "", fmt.Errorf("timeout")
	case <-ctxPing.Done():
		return time.Since(startTime), "", fmt.Errorf("user cancelled")
	case <-s.Context().Done():
		return time.Since(startTime), "", fmt.Errorf("netceptor shutdown")
	}
}

type NetcForTraceroute interface {
	MaxForwardingHops() byte
	Ping(ctx context.Context, target string, hopsToLive byte) (time.Duration, string, error)
	Context() context.Context
}

// TracerouteResult is the result of one hop of a traceroute.
type TracerouteResult struct {
	From string
	Time time.Duration
	Err  error
}

func (s *Netceptor) Traceroute(ctx context.Context, target string) <-chan *TracerouteResult {
	return CreateTraceroute(ctx, s, target)
}

// Traceroute returns a channel which will receive a series of hops between this node and the target.
func CreateTraceroute(ctx context.Context, s NetcForTraceroute, target string) <-chan *TracerouteResult {
	results := make(chan *TracerouteResult)
	go func() {
		defer close(results)
		for i := 0; i <= int(s.MaxForwardingHops()); i++ {
			pingTime, pingRemote, err := s.Ping(ctx, target, byte(i))
			res := &TracerouteResult{
				From: pingRemote,
				Time: pingTime,
			}
			if err != nil && err.Error() != ProblemExpiredInTransit {
				res.Err = err
			}
			select {
			case results <- res:
			case <-ctx.Done():
				return
			case <-s.Context().Done():
				return
			}
			if res.Err != nil || err == nil {
				return
			}
		}
	}()

	return results
}
