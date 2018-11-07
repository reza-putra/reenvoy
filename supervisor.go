package reenvoy

import "fmt"

type Incrementor struct {
	current int
	next    chan int
	stop    chan bool
}

func (i *Incrementor) Stop() {
	fmt.Println("Stopping the service")
	i.stop <- true
}

func (i *Incrementor) Serve() {
	for {
		select {
		case i.next <- i.current:
			i.current++
		case <-i.stop:
			// We sync here just to guarantee the output of "Stopping the service",
			// so this passes the test reliably.
			// Most services would simply "return" here.
			i.stop <- true
			return
		}
	}

}
