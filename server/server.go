package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"uk.ac.bris.cs/gameoflife/gol"
)

func getOutboundIP() string {
	conn, _ := net.Dial("udp", "8.8.8.8:80")
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr).IP.String()
	return localAddr
}

func worker(startY, endY, startX, endX, Height, Width int, world [][]byte, out chan<- [][]byte) {
	SliceHeight := endY - startY
	SliceWidth := endX - startX
	NewPart := make([][]byte, SliceHeight)
	for i := range NewPart {
		NewPart[i] = make([]byte, SliceWidth)
	}
	for y := startY; y < endY; y++ {
		for x := startX; x < endX; x++ {
			NewVal := judge(x, y, world, Width, Height)
			NewPart[y-startY][x-startX] = NewVal
		}
	}
	out <- NewPart
}

func transpose(work [][]byte) [][]byte {
	NewWork := make([][]byte, len(work[0]))
	for i := range NewWork {
		NewWork[i] = make([]byte, len(work))
	}
	for y := 0; y < len(work); y++ {
		for x := 0; x < len(work[0]); x++ {
			NewWork[x][y] = work[y][x]
		}
	}
	return NewWork
}

func judge(targetX int, targetY int, world [][]byte, width int, height int) byte {
	count := 0
	var NewVal byte
	for j := -1; j <= 1; j++ {
		for i := -1; i <= 1; i++ {
			y := (targetY + height + j) % height
			x := (targetX + width + i) % width
			if world[y][x] == 255 {
				count += 1
			}
		}
	}
	if world[targetY][targetX] == 255 {
		count -= 1
		if count < 2 || count > 3 {
			NewVal = 0
		} else {
			NewVal = 255
		}

	} else {
		if count == 3 {
			NewVal = 255
		} else {
			NewVal = 0
		}
	}

	return NewVal
}

type MyOperations struct{}

func (s *MyOperations) Kill(req gol.ControlRequest, res *gol.ControlResponse) (err error) {
	os.Exit(0)
	return
}
func (s *MyOperations) Worker(req gol.WorkRequest, res *gol.WorkResponse) (err error) {
	StartY := req.WorkItem.StartY
	EndY := req.WorkItem.EndY
	//StartX := req.WorkItem.StartX
	//EndX := req.WorkItem.EndX
	//SliceHeight := EndY - StartY
	//SliceWidth := EndX - StartX
	world := req.WorkItem.World
	Height := req.WorkItem.P.ImageHeight
	Width := req.WorkItem.P.ImageWidth
	Threads := req.WorkItem.P.Threads

	//Threads := 1
	workerWidth := Width / Threads
	out := make([]chan [][]byte, Threads)
	for i := range out {
		out[i] = make(chan [][]byte)
	}
	for i := 0; i < Threads; i++ {
		work := world
		w1 := i * workerWidth
		var w2 int
		if i == Threads-1 {
			w2 = Width
		} else {
			w2 = (i + 1) * workerWidth
		}
		go worker(StartY, EndY, w1, w2, Height, Width, work, out[i])
	}
	NewPart := make([][]byte, 0)
	for i := range NewPart {
		NewPart[i] = make([]byte, 0)
	}
	for i := 0; i < Threads; i++ {
		part := <-out[i]
		NewPart = append(NewPart, transpose(part)...)
	}
	res.Part = transpose(NewPart)
	if err != nil {
		fmt.Println(err)
	}
	return
}

func main() {

	pAddr := flag.String("port", "8090", "Port to listen on")
	flag.Parse()
	rpc.Register(&MyOperations{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
