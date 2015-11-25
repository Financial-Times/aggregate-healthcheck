package main

type RingBuffer struct {
	inputChannel  <-chan *HealthTimed
	outputChannel chan *HealthTimed
}

func NewRingBuffer(inputChannel <-chan *HealthTimed, outputChannel chan *HealthTimed) *RingBuffer {
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