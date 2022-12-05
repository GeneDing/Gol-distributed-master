package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/gol"
)

var Init gol.WorkDone
var mutex sync.Mutex
var StateStore Store
var Current gol.WorkDone
var Final gol.WorkDone
var ServerAddress = make([]string, 4)
var pause chan bool

type Store struct {
	sync.RWMutex
	m      map[int]gol.WorkDone
	status string
}

type Broker struct {
}

func worker(Client *rpc.Client, startY, endY, startX, endX int, work [][]byte, p gol.Params, out chan<- [][]byte) {
	WorkItem := gol.Item{StartY: startY, EndY: endY, StartX: startX, EndX: endX, World: work, P: p}
	WorkRequest := gol.WorkRequest{WorkItem: WorkItem}
	WorkResponse := new(gol.WorkResponse)
	Client.Call("MyOperations.Worker", WorkRequest, WorkResponse)
	out <- WorkResponse.Part
}

func process(Clients []*rpc.Client, world [][]byte, p gol.Params, start, end, nodes int, ch chan gol.WorkDone) {
	for j := start; j <= end; j++ {
		workerHeight := p.ImageHeight / nodes
		out := make([]chan [][]byte, nodes)
		for i := range out {
			out[i] = make(chan [][]byte)
		}
		for i := 0; i < nodes; i++ {
			work := world
			h1 := i * workerHeight
			var h2 int
			if i == nodes-1 {
				h2 = p.ImageHeight
			} else {
				h2 = (i + 1) * workerHeight
			}
			go worker(Clients[i], h1, h2, 0, p.ImageWidth, work, p, out[i])
		}
		NewWorld := make([][]byte, 0)
		for i := range NewWorld {
			NewWorld[i] = make([]byte, 0)
		}
		for i := 0; i < nodes; i++ {
			part := <-out[i]
			NewWorld = append(NewWorld, part...)
		}
		world = NewWorld
		NewCurrent := gol.WorkDone{World: NewWorld, CompletedTurn: j}
		Current = NewCurrent
		StateStore.Lock()
		StateStore.m[j] = NewCurrent
		CurrentStatus := StateStore.status
		StateStore.Unlock()
		switch CurrentStatus {
		case "executing":
			continue
		case "pause":
			<-pause
		case "shutdown":
			ControlReq := gol.ControlRequest{}
			ControlRes := new(gol.ControlResponse)
			for i := 0; i < nodes; i++ {
				Clients[i].Call("MyOperations.Kill", ControlReq, ControlRes)
			}
			os.Exit(0)
		}
	}
	ch <- gol.WorkDone{CompletedTurn: end, World: world}
}
func (b *Broker) CheckCurrentState(req gol.CheckRequest, res *gol.CheckResponse) (err error) {
	res.State = Current
	return err
}

func (b *Broker) GetWorld(req gol.GetRequest, res *gol.GetResponse) (err error) {
	timer := time.NewTimer(1 * time.Millisecond)
	<-timer.C
	StateStore.RLock()
	res.Done = StateStore.m[req.Target]
	delete(StateStore.m, req.Target)
	StateStore.RUnlock()
	return
}

func (b *Broker) Ctrl(req gol.ControlRequest, res *gol.ControlResponse) (err error) {
	StateStore.RLock()
	Status := StateStore.status
	StateStore.RUnlock()
	var NewStatus string
	if Status == "pause" && req.Message == "pause" {
		NewStatus = "executing"
		pause <- true
	} else {
		NewStatus = req.Message
	}
	StateStore.Lock()
	StateStore.status = NewStatus
	StateStore.Unlock()
	res.Message = NewStatus
	return
}

func (b *Broker) Publish(req gol.PublishRequest, res *gol.PublishResponse) (err error) {
	var Clients = make([]*rpc.Client, 4)
	pause = make(chan bool)
	nodes := 4
	if StateStore.status == "" {
		StateStore = Store{m: make(map[int]gol.WorkDone)}
	}
	//InitWork := gol.WorkDone{World: req.World, CompletedTurn: 0}
	//Current = InitWork
	world := req.World
	p := req.P
	ch := make(chan gol.WorkDone)
	StateStore.Lock()
	StateStore.status = "executing"
	StateStore.Unlock()
	for k := 0; k < nodes; k++ {
		Client, e := rpc.Dial("tcp", ServerAddress[k])
		if e != nil {
			fmt.Println(e)
		} else {
			Clients[k] = Client
		}
	}

	go process(Clients, world, p, 1, p.Turns, nodes, ch)

	res.Done = <-ch

	for k := 0; k < nodes; k++ {
		Clients[k].Close()
	}

	return err
}

func main() {
	ServerAddress[0] = "127.0.0.1:8050"
	ServerAddress[1] = "127.0.0.1:8060"
	ServerAddress[2] = "127.0.0.1:8070"
	ServerAddress[3] = "127.0.0.1:8080"

	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rpc.Register(&Broker{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
