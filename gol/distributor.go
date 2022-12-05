package gol

import (
	"fmt"
	"net/rpc"
	"os"
	"sync"
	"time"

	//"time"
	"uk.ac.bris.cs/gameoflife/util"
)

var Init WorkDone
var pause chan bool

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
	keypress   <-chan rune
}

func countAliveCells(world [][]byte, p Params) []util.Cell {
	NewAlive := make([]util.Cell, 0)
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			if world[y][x] == 255 {
				NewAlive = append(NewAlive, util.Cell{X: x, Y: y})
			}
		}
	}
	return NewAlive
}
func sdlLive(client *rpc.Client, p Params, world [][]byte, c distributorChannels, wg *sync.WaitGroup) {
	work := world
	current := 1
	timer := time.NewTimer(10 * time.Millisecond)
	select {
	case <-timer.C:
		for i := 1; i <= p.Turns; i++ {
			getRequest := GetRequest{Target: i}
			getResponse := new(GetResponse)
			client.Call("Broker.GetWorld", getRequest, getResponse)
			done := getResponse.Done
			NewWorld := done.World
			Turn := done.CompletedTurn
			for y := 0; y < p.ImageHeight; y++ {
				for x := 0; x < p.ImageWidth; x++ {
					if work[y][x] != NewWorld[y][x] {
						c.events <- CellFlipped{CompletedTurns: Turn, Cell: util.Cell{X: x, Y: y}}
					}
				}
			}
			work = NewWorld
			c.events <- TurnComplete{Turn + 1}
			current++
		}
	}
	wg.Done()
	return
}

func regularcheck(client *rpc.Client, p Params, c distributorChannels, q chan bool, wg *sync.WaitGroup) {
	ticker := time.NewTicker(2 * time.Second)
	//var lastpressed rune
	for {
		select {
		case <-ticker.C:
			checkRequest := CheckRequest{}
			checkResponse := new(CheckResponse)
			client.Call(AccessHandler, checkRequest, checkResponse)
			CurrentAlive := countAliveCells(checkResponse.State.World, p)
			c.events <- AliveCellsCount{CompletedTurns: checkResponse.State.CompletedTurn, CellsCount: len(CurrentAlive)}
		case k := <-c.keypress:
			checkRequest := CheckRequest{}
			checkResponse := new(CheckResponse)
			err := client.Call(AccessHandler, checkRequest, checkResponse)
			if err != nil {
				fmt.Println(err)
			}
			client.Call(AccessHandler, checkRequest, checkResponse)
			switch k {
			case 's':
				{
					filename := fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, checkResponse.State.CompletedTurn)
					c.ioCommand <- ioOutput
					c.ioFilename <- filename
					for y := 0; y < p.ImageHeight; y++ {
						for x := 0; x < p.ImageWidth; x++ {
							c.ioOutput <- checkResponse.State.World[y][x]
						}
					}
					c.events <- ImageOutputComplete{CompletedTurns: checkResponse.State.CompletedTurn, Filename: filename}
				}
			case 'q':
				{
					filename := fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, checkResponse.State.CompletedTurn)
					c.ioCommand <- ioOutput
					c.ioFilename <- filename
					for y := 0; y < p.ImageHeight; y++ {
						for x := 0; x < p.ImageWidth; x++ {
							c.ioOutput <- checkResponse.State.World[y][x]
						}
					}
					c.events <- ImageOutputComplete{CompletedTurns: checkResponse.State.CompletedTurn, Filename: filename}
					os.Exit(0)
				}
			case 'k':
				filename := fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, checkResponse.State.CompletedTurn)
				c.ioCommand <- ioOutput
				c.ioFilename <- filename
				for y := 0; y < p.ImageHeight; y++ {
					for x := 0; x < p.ImageWidth; x++ {
						c.ioOutput <- checkResponse.State.World[y][x]
					}
				}
				c.events <- ImageOutputComplete{CompletedTurns: checkResponse.State.CompletedTurn, Filename: filename}
				ctrlReq := ControlRequest{Message: "shutdown"}
				ctrlRes := new(ControlResponse)
				client.Call("Broker.Ctrl", ctrlReq, ctrlRes)
				os.Exit(0)
			case 'p':
				filename := fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, checkResponse.State.CompletedTurn)
				c.ioCommand <- ioOutput
				c.ioFilename <- filename
				for y := 0; y < p.ImageHeight; y++ {
					for x := 0; x < p.ImageWidth; x++ {
						c.ioOutput <- checkResponse.State.World[y][x]
					}
				}
				c.events <- ImageOutputComplete{CompletedTurns: checkResponse.State.CompletedTurn, Filename: filename}
				ctrlReq := ControlRequest{Message: "pause"}
				ctrlRes := new(ControlResponse)
				client.Call("Broker.Ctrl", ctrlReq, ctrlRes)
				for {
					select {
					case newkey := <-c.keypress:
						//filename2 := fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, checkResponse.State.CompletedTurn)

						if newkey == 'p' {
							ctrlReq2 := ControlRequest{Message: "pause"}
							ctrlRes2 := new(ControlResponse)
							client.Call("Broker.Ctrl", ctrlReq2, ctrlRes2)
							break
						} else if newkey == 'k' {
							ctrlReq2 := ControlRequest{Message: "kill"}
							ctrlRes2 := new(ControlResponse)
							c.events <- ImageOutputComplete{CompletedTurns: checkResponse.State.CompletedTurn, Filename: filename}
							client.Call("Broker.Ctrl", ctrlReq2, ctrlRes2)
							os.Exit(0)
						}
					default:
						time.Sleep(500 * time.Millisecond)
					}
				}
			}
		case <-q:
			wg.Done()
			return
		}
	}
}
func makecall(client *rpc.Client, world [][]byte, p Params, c distributorChannels, FinalWorld chan [][]byte, wg *sync.WaitGroup) {
	if p.Turns > 0 {
		publishRequest := PublishRequest{World: world, P: p}
		publishResponse := new(PublishResponse)
		client.Call("Broker.Publish", publishRequest, publishResponse)
		//time.Sleep(1 * time.Millisecond)
		world = publishResponse.Done.World
	}
	wg.Done()
	FinalWorld <- world
	return
	//quit <- true

}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {
	// TODO: Create a 2D slice to store the world.
	pause = make(chan bool)
	var AliveCells []util.Cell
	quit := make(chan bool)
	c.ioCommand <- ioInput
	c.ioFilename <- fmt.Sprintf("%vx%v", p.ImageWidth, p.ImageHeight)
	world := make([][]byte, p.ImageHeight)
	for i := range world {
		world[i] = make([]byte, p.ImageWidth)
	}

	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			val := <-c.ioInput
			world[y][x] = val
			if val == 255 {
				c.events <- CellFlipped{CompletedTurns: 1, Cell: util.Cell{X: x, Y: y}}
			}
		}
	}
	Init = WorkDone{CompletedTurn: 0, World: world}
	//brokerAddr := flag.String("broker", "127.0.0.1:8030", "IP:port string to connect to as server")
	//flag.Parse()
	client, _ := rpc.Dial("tcp", "127.0.0.1:8030")
	defer client.Close()
	FinalWorld := make(chan [][]byte)
	if p.Turns > 0 {
		var wg sync.WaitGroup

		wg.Add(3)
		go regularcheck(client, p, c, quit, &wg)
		go makecall(client, world, p, c, FinalWorld, &wg)
		go sdlLive(client, p, world, c, &wg)
		world = <-FinalWorld
		quit <- true
		wg.Wait()
	}
	//
	//}
	c.ioCommand <- ioOutput
	c.ioFilename <- fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, p.Turns)
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x]
			if world[y][x] == 255 {
				AliveCells = append(AliveCells, util.Cell{X: x, Y: y})
			}
		}
	}
	c.events <- FinalTurnComplete{p.Turns, AliveCells}
	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{p.Turns, Quitting}
	close(c.events)
	// TODO: Execute all turns of the Game of Life.

	// TODO: Report the final state using FinalTurnCompleteEvent.

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.

}
