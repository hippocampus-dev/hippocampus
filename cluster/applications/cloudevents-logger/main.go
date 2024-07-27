package main

import (
	"context"
	"log"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

func handle(event cloudevents.Event) cloudevents.Result {
	log.Print(event)

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
