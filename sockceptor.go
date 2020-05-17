package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/jupp0r/go-priority-queue"
	"io"
	"math"
	"net"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"
)

type stringList []string

func (i *stringList) String() string {
	return strings.Join(*i, ", ")
}

func (i *stringList) Set(value string) error {
	*i = append(*i, value)
	return nil
}

var nodeId string
var debugFlag bool
var peers stringList
var listeners stringList
var udpServices stringList

var epoch int64
var sequence int64

var connectionLock = &sync.RWMutex{}
type ConnInfo struct {
	readChan  chan []byte
	writeChan chan []byte
	errorChan chan error
}
var connections = make(map[string]ConnInfo)
var seenUpdates = make(map[string]time.Time)
type nodeInfo struct {
	epoch int64
	sequence int64
}
var knownNodeInfo = make(map[string]nodeInfo)
var knownConnections = make(map[string]map[string]float64)
var routingTable = make(map[string]string)

type RoutingUpdate struct {
	NodeId         string
	UpdateId       string
	UpdateEpoch    int64
	UpdateSequence int64
	Connections    []string
}
var sendRouteBroadcastChan = make(chan bool)
var updateRoutingTableChan = make(chan bool)

type MessageData struct {
	FromNode    string
	FromService string
	ToNode      string
	ToService   string
	Data        []byte
}

var serviceRegistry = make(map[string]chan MessageData)

func debug(format string, a ...interface{}) {
	if debugFlag {
		fmt.Printf(format, a...)
	}
}

func updateRoutingTable() {
	connectionLock.Lock()
	defer connectionLock.Unlock()
	// Dijkstra's algorithm
	Q := pq.New()
	Q.Insert(nodeId, 0.0)
	cost := make(map[string]float64)
	prev := make(map[string]string)
	for node := range knownConnections {
		if node == nodeId {
			cost[node] = 0.0
		} else {
			cost[node] = math.MaxFloat64
		}
		prev[node] = ""
		Q.Insert(node, cost[node])
	}
	for Q.Len() > 0 {
		nodeIf, _ := Q.Pop()
		node := fmt.Sprintf("%v", nodeIf)
		for neighbor, edgeCost := range knownConnections[node] {
			pathCost := cost[node] + edgeCost
			if pathCost < cost[neighbor] {
				cost[neighbor] = pathCost
				prev[neighbor] = node
				Q.Insert(neighbor, pathCost)
			}
		}
	}
	routingTable = make(map[string]string)
	for dest := range knownConnections {
		p := dest
		for {
			if prev[p] == nodeId {
				routingTable[dest] = p
				break
			} else if prev[p] == "" {
				break
			}
			p = prev[p]
		}
	}
	go printRoutingTable()
}

func routingTableUpdater() {
	updateRequested := false
	nextUpdateTime := time.Now().Add(time.Hour)
	for {
		select {
		case <- time.After(time.Until(nextUpdateTime)):
			nextUpdateTime = time.Now().Add(time.Hour)
			if updateRequested {
				updateRequested = false
				updateRoutingTable()
			}
		case <- updateRoutingTableChan:
			proposedTime := time.Now().Add(time.Second * 5)
			updateRequested = true
			if proposedTime.Before(nextUpdateTime) {
				nextUpdateTime = proposedTime
			}
		}
	}
}


func broadcast(message []byte, excludeconn string) {
	connectionLock.RLock()
	writeChans := make([]chan []byte, 0)
	for conn, connInfo := range connections {
		if conn != excludeconn {
			writeChans = append(writeChans, connInfo.writeChan)
		}
	}
	connectionLock.RUnlock()
	for i := range writeChans {
		i := i
		go func() { writeChans[i] <- message }()
	}
}

func randomString(minbits int) (string) {
	randbytes := make([]byte, int(math.Ceil(float64(minbits)/8)))
	_, _ = io.ReadFull(rand.Reader, randbytes)
	str := base64.StdEncoding.EncodeToString(randbytes)
	return strings.TrimRight(str, "=")
}

func makeMessage(command string, data interface{}) ([]byte, error) {
	dataj, err := json.Marshal(data)
	if err != nil { return []byte{}, nil }
	msg := []byte(command)
	msg = append(msg, ' ')
	msg = append(msg, dataj...)
	return msg, nil
}

func forwardMessage(md MessageData) error {
	nextHop, ok := routingTable[md.ToNode]
	if ! ok { return fmt.Errorf("no route to node") }
	connectionLock.RLock()
	writeChan := connections[nextHop].writeChan
	connectionLock.RUnlock()
	debug("Forwarding message to %s via %s\n", md.ToNode, nextHop)
	if writeChan != nil {
		message, err := makeMessage("send", md)
		if err != nil { return err }
		writeChan <- message
		return nil
	} else {
		return fmt.Errorf("could not write to node")
	}
}

func sendMessage(fromService string, toNode string, toService string, data []byte) error {
	md := MessageData{
		FromNode:    nodeId,
		FromService: fromService,
		ToNode:      toNode,
		ToService:   toService,
		Data:        data,
	}
	if toNode == nodeId {
		svcChan, ok := serviceRegistry[toService]
		if ok {
			svcChan <- md
		}
		return nil
	} else {
		return forwardMessage(md)
	}
}

func printRoutingTable() {
	if ! debugFlag { return }
	connectionLock.RLock()
	defer connectionLock.RUnlock()
	debug("Known Connections:\n")
	for conn := range knownConnections {
		debug("   %s: ", conn)
		for peer := range knownConnections[conn] {
			debug("%s ", peer)
		}
		debug("\n")
	}
	debug("Routing Table:\n")
	for node := range routingTable {
		debug("   %s via %s\n", node, routingTable[node])
	}
	debug("\n")
}

func sendRoutingUpdate() error {
	sequence += 1
	connectionLock.RLock()
	conns := make([]string, len(connections))
	i := 0
	for conn := range connections {
		conns[i] = conn
		i++
	}
	connectionLock.RUnlock()
	update := RoutingUpdate{
		NodeId:         nodeId,
		UpdateId:       randomString(128),
		UpdateEpoch:    epoch,
		UpdateSequence: sequence,
		Connections:    conns,
	}
	message, err := makeMessage("route", update)
	if err != nil { return err }
	broadcast(message, "")
	return nil
}

func routeSender() {
	nextUpdateTime := time.Now().Add(time.Second * 5)
	for {
		select {
		case <- time.After(time.Until(nextUpdateTime)):
			_ = sendRoutingUpdate()
			nextUpdateTime = time.Now().Add(time.Second * 10)
		case <- sendRouteBroadcastChan:
			proposedTime := time.Now().Add(time.Second)
			if proposedTime.Before(nextUpdateTime) {
				nextUpdateTime = proposedTime
			}
		}
	}
}

func handleRoutingUpdate(data []byte, recvConn string) {
	ri := RoutingUpdate{}
	err := json.Unmarshal(data, &ri)
	if err != nil { return }
	if ri.NodeId == nodeId { return }
	connectionLock.RLock()
	_, ok := seenUpdates[ri.UpdateId]
	connectionLock.RUnlock()
	if ok { return }
	connectionLock.Lock()
	seenUpdates[ri.UpdateId] = time.Now()
	ni, ok := knownNodeInfo[ri.NodeId]
	connectionLock.Unlock()
	if ok {
		if ri.UpdateEpoch < ni.epoch { return }
		if ri.UpdateEpoch == ni.epoch && ri.UpdateSequence <= ni.sequence { return }
	} else {
		sendRouteBroadcastChan <- true
		ni = nodeInfo{}
	}
	ni.epoch = ri.UpdateEpoch
	ni.sequence = ri.UpdateSequence
	conns := make(map[string]float64)
	for conn := range ri.Connections {
		conns[ri.Connections[conn]] = 1.0
	}
	connectionLock.Lock()
	changed := false
	if ! reflect.DeepEqual(conns, knownConnections[ri.NodeId]) {
		changed = true
	}
	knownNodeInfo[ri.NodeId] = ni
	knownConnections[ri.NodeId] = conns
	for conn := range knownConnections {
		_, ok = conns[conn]
		if ! ok {
			delete(knownConnections[conn], ri.NodeId)
		}
	}
	connectionLock.Unlock()
	message, err := makeMessage("route", ri)
	if err != nil { return }
	broadcast(message, recvConn)
	if changed {
		updateRoutingTableChan <- true
	}
}

func handleSend(data []byte) error {
	md := MessageData{}
	err := json.Unmarshal(data, &md)
	if err != nil { return err }
	if md.ToNode == nodeId {
		svcChan, ok := serviceRegistry[md.ToService]
		if ok {
			svcChan <- md
			return nil
		} else {
			return fmt.Errorf("unknown service")
		}
	} else {
		return forwardMessage(md)
	}
}

func protoHello(rw *bufio.ReadWriter) (string, error) {
	_, err := rw.WriteString(nodeId)
	if err != nil {
		return "", err
	}
	_, err = rw.WriteString("\n")
	if err != nil {
		return "", err
	}
	err = rw.Flush()
	if err != nil {
		return "", err
	}
	remoteNodeId, err := rw.ReadString('\n')
	return strings.TrimSuffix(remoteNodeId, "\n"), err
}

func protoReader(r *bufio.Reader, readChan chan []byte, errorChan chan error) {
	for {
		message, err := r.ReadBytes('\n')
		if err != nil {
			errorChan <- err
			return
		}
		readChan <- bytes.TrimSuffix(message, []byte("\n"))
	}
}

func protoWriter(w *bufio.Writer, writeChan chan []byte, errorChan chan error) {
	for {
		message, more := <- writeChan
		if !more {
			return
		}
		_, err := w.Write(message)
		if err != nil {
			errorChan <- err
			return
		}
		if !bytes.HasSuffix(message, []byte("\n")) {
			_, err := w.Write([]byte("\n"))
			if err != nil {
				errorChan <- err
				return
			}
		}
		err = w.Flush()
		if err != nil {
			errorChan <- err
			return
		}
	}
}

func runProtocol(conn net.Conn) {
	defer func() {
		_ = conn.Close()
	}()
	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	remoteNodeId, err := protoHello(rw)
	if err != nil {
		return
	}
	ci := ConnInfo{
		make(chan []byte),
		make(chan []byte),
		make(chan error),
	}
	defer func() {
		close(ci.readChan)
		close(ci.writeChan)
		close(ci.errorChan)
	}()
	connectionLock.Lock()
	connections[remoteNodeId] = ci
	_, ok := knownConnections[nodeId]
	if ! ok {
		knownConnections[nodeId] = make(map[string]float64)
	}
	knownConnections[nodeId][remoteNodeId] = 1.0
	connectionLock.Unlock()
	sendRouteBroadcastChan <- true
	defer func() {
		connectionLock.Lock()
		delete(connections, remoteNodeId)
		delete(knownConnections[remoteNodeId], nodeId)
		delete(knownConnections[nodeId], remoteNodeId)
		connectionLock.Unlock()
		sendRouteBroadcastChan <- true
	}()
	go protoReader(rw.Reader, ci.readChan, ci.errorChan)
	go protoWriter(rw.Writer, ci.writeChan, ci.errorChan)
	for {
		select {
		case message := <- ci.readChan:
			msgparts := bytes.SplitN(message, []byte(" "), 2)
			command := string(msgparts[0])
			data := []byte("")
			if len(msgparts) > 1 {
				data = msgparts[1]
			}
			if command == "route" {
				handleRoutingUpdate(data, remoteNodeId)
			} else if command == "send" {
				_ = handleSend(data)
			} else {
				debug("Unknown command: %s, Data: %s\n", command, data)
			}
		case err = <- ci.errorChan:
			return
		}
	}
}

func runPeerConnection(wg *sync.WaitGroup, peer string) {
	defer wg.Done()
	reconnectDelay := 1.0
	for {
		conn, err := net.Dial("tcp", peer)
		if err == nil {
			reconnectDelay = 1.0
			runProtocol(conn)
		} else {
			time.Sleep(time.Duration(reconnectDelay) * time.Second)
			reconnectDelay = math.Min(reconnectDelay * 2.0, 60.0)
		}
	}
}

func runListener(wg *sync.WaitGroup, listener string) {
	defer wg.Done()
	ln, err := net.Listen("tcp", listener)
	if err != nil { panic(err) }
	for {
		conn, _ := ln.Accept()
		go runProtocol(conn)
	}
}

func runUdpService(wg *sync.WaitGroup, direction string, lservice string, host string,
	port string, node string, rservice string) {
	defer wg.Done()
	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%s", host, port))
	if err != nil { panic(err) }
	var udpConn *net.UDPConn
	if direction == "in" {
		udpConn, err = net.ListenUDP("udp", udpAddr)
		if err != nil { panic(err) }
	} else {
		udpConn, err = net.DialUDP("udp", nil, udpAddr)
		if err != nil { panic(err) }
	}
	defer func() { _ = udpConn.Close() }()
	serviceChan := make(chan MessageData)
	addrChan := make(chan net.Addr)
	serviceRegistry[lservice] = serviceChan
	go func(serviceChan chan MessageData, addrChan chan net.Addr) {
		var addr net.Addr
		for {
			select {
			case newAddr := <- addrChan:
				addr = newAddr
			case md := <-serviceChan:
				if direction == "in" {
					_, err = udpConn.WriteTo(md.Data, addr)
				} else {
					_, err = udpConn.Write(md.Data)
				}
				if err != nil {
					panic(err)
				}
			}
		}
	}(serviceChan, addrChan)
	for {
		buffer := make([]byte, 1<<16)
		n, addr, err := udpConn.ReadFrom(buffer)
		if err != nil { panic(err) }
		debug("UDP from %s data len %d\n", addr, n)
		addrChan <- addr
		_ = sendMessage(lservice, node, rservice, buffer[:n])
	}
}

func main() {
	flag.StringVar(&nodeId, "node-id", "", "local node ID")
	flag.BoolVar(&debugFlag, "debug", false, "show debug output")
	flag.Var(&peers, "peer", "host:port  to connect outbound to")
	flag.Var(&listeners, "listen", "host:port to listen on for peer connections")
	flag.Var(&udpServices, "udp", "{in|out}:lservice:host:port:node:rservice")
	flag.Parse()
	if nodeId == "" {
		println("Must specify a node ID")
		os.Exit(1)
	}
	var wg sync.WaitGroup
	epoch = time.Now().Unix()
	sequence = 0
	debug("Starting as node ID %s\n", nodeId)
	go routeSender()
	go routingTableUpdater()
	for _, listener := range listeners {
		debug("Running listener %s\n", listener)
		go runListener(&wg, listener)
		wg.Add(1)
	}
	for _, peer := range peers {
		debug("Running peer connection %s\n", peer)
		go runPeerConnection(&wg, peer)
		wg.Add(1)
	}
	for _, udpService := range udpServices {
		debug("Running UDP service %s\n", udpService)
		params := strings.Split(udpService, ":")
		if len(params) != 6 { panic("Invalid parameters for udp service") }
		go runUdpService(&wg, params[0], params[1], params[2], params[3], params[4], params[5])
		wg.Add(1)
	}
	debug("Initialization complete\n")
	wg.Wait()
}
