package gol

// Params provides the details of how to run the Game of Life and which image to load.
var Handler = "MyOperations.Start"
var AccessHandler = "Broker.CheckCurrentState"

type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

type WorkDone struct {
	CompletedTurn int
	World         [][]byte
}

type Item struct {
	StartY int
	EndY   int
	StartX int
	EndX   int
	World  [][]byte
	P      Params
}

type CheckRequest struct {
}

type CheckResponse struct {
	State WorkDone
}

type GetRequest struct {
	Target int
}

type GetResponse struct {
	Done WorkDone
}

type ControlRequest struct {
	Message string
}
type ControlResponse struct {
	Message string
}

type PublishRequest struct {
	World [][]byte
	P     Params
}

type PublishResponse struct {
	Number int
	Done   WorkDone
}

type WorkRequest struct {
	WorkItem Item
}
type WorkResponse struct {
	Part [][]byte
}

// Run starts the processing of Game of Life. It should initialise channels and goroutines.
func Run(p Params, events chan<- Event, keyPresses <-chan rune) {

	//	TODO: Put the missing channels in here.

	ioCommand := make(chan ioCommand)
	ioIdle := make(chan bool)
	ioFilename := make(chan string)
	ioOutput := make(chan uint8)
	ioInput := make(chan uint8)

	ioChannels := ioChannels{
		command:  ioCommand,
		idle:     ioIdle,
		filename: ioFilename,
		output:   ioOutput,
		input:    ioInput,
	}
	go startIo(p, ioChannels)

	distributorChannels := distributorChannels{
		events:     events,
		ioCommand:  ioCommand,
		ioIdle:     ioIdle,
		ioFilename: ioFilename,
		ioOutput:   ioOutput,
		ioInput:    ioInput,
		keypress:   keyPresses,
	}

	distributor(p, distributorChannels)
}
