package reenvoy

import (
	"fmt"
	"testing"

	"github.com/thejerf/suture"
)

func Test_Supervisor(t *testing.T) {
	supervisor := suture.NewSimple("Supervisor")
	service := &Incrementor{0, make(chan int), make(chan bool)}
	supervisor.Add(service)

	supervisor.ServeBackground()

	fmt.Println("Got:", <-service.next)
	fmt.Println("Got:", <-service.next)
	supervisor.Stop()

	// We sync here just to guarantee the output of "Stopping the service"
	<-service.stop
}
