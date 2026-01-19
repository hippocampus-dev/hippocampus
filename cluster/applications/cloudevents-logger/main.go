package main

import (
	"context"
	"fmt"
	"log"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

func handle(event cloudevents.Event) cloudevents.Result {
	fmt.Println(event)

	return cloudevents.ResultACK
}

func main() {
	c, err := cloudevents.NewDefaultClient()
	if err != nil {
		log.Fatalf("failed to create client: %+v", err)
	}

	if err := c.StartReceiver(context.Background(), handle); err != nil {
		log.Fatalf("failed to start receiver: %+v", err)
	}
}
