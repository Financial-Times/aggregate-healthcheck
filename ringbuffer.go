package main

type RingBuffer struct {
	inputChannel  <-chan int
	outputChannel chan int
}

func NewRingBuffer(inputChannel <-chan int, outputChannel chan int) *RingBuffer {
	return &RingBuffer{inputChannel, outputChannel}
}

func (r *RingBuffer) Run() {
	for v := range r.inputChannel {
		select {
		case r.outputChannel <- v:
		default:
			<-r.outputChannel
			r.outputChannel <- v
		}
	}
	close(r.outputChannel)
}