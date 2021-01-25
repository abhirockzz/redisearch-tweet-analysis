package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	svc "github.com/abhirockzz/redisearch-go-app/index"
	"github.com/abhirockzz/redisearch-go-app/twitter"
)

func main() {
	stream := twitter.StartStream()

	exit := make(chan os.Signal)
	signal.Notify(exit, syscall.SIGINT, syscall.SIGTERM)
	<-exit

	stream.Stop()
	fmt.Println("stream stopped")
	svc.Close()
}
