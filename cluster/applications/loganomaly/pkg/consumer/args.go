package consumer

import "time"

type Args struct {
	BootstrapServers    string        `validate:"required"`
	InputTopic          string        `validate:"required"`
	OutputTopic         string        `validate:"required"`
	ZScoreThreshold     float64       `validate:"gt=0"`
	EvaluationInterval  time.Duration `validate:"gt=0"`
	SuppressionDuration time.Duration `validate:"gt=0"`
}

func DefaultArgs() *Args {
	return &Args{
		BootstrapServers:    "eventing-kafka-bootstrap.knative-eventing.svc.cluster.local:9092",
		InputTopic:          "loganomaly-logs",
		OutputTopic:         "loganomaly-events",
		ZScoreThreshold:     3.0,
		EvaluationInterval:  30 * time.Second,
		SuppressionDuration: 5 * time.Minute,
	}
}
