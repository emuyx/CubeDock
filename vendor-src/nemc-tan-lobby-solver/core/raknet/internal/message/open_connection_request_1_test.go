package message

import (
	"fmt"
	"sync"
	"testing"
)

func TestOpenConnectionRequest1ConcurrentMarshal(t *testing.T) {
	const workers = 32
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for iteration := 0; iteration < 200; iteration++ {
				packet := &OpenConnectionRequest1{
					ClientProtocol: byte(worker + 1),
					MTU:            uint16(576 + (worker+iteration)%64),
				}
				if _, err := packet.MarshalBinary(); err != nil {
					errs <- fmt.Errorf("worker %d: %w", worker, err)
					return
				}
			}
		}(worker)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}
