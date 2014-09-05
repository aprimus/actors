// stringenerator creates a channel which will produce
// unique strings on each read.
package stringgenerator

import (
	"strconv"
	"sync"
)

// NewGenerator creates a GUID channel, from which each
// read contains a new, unique (per generator) string.
// Sending a message on the second returned channel (bool)
// will cause the generator to terminate.
func NewGenerator(prefix string, wg sync.WaitGroup) (chan string, chan bool) {
	ch := make(chan string, 5)
	control := make(chan bool, 1)
	go func() {
		defer wg.Done()
		wg.Add(1)
		generator(prefix, ch, control)
	}()
	return ch, control
}

func generator(prefix string, ch chan string, control chan bool) {
	counter := 0
	for {
		select {
		case <-control:
			return
		default:
			s := prefix + "-" + strconv.Itoa(counter)
			ch <- s
			counter++
		}
	}
}
