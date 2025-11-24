package lock

import "strings"

type Args struct {
	LockMode          string   `validate:"required,oneof=redlock etcd"`
	RedisAddresses    []string `validate:"required,omitempty"`
	EtcdAddresses     []string `validate:"required,omitempty"`
	QueueRedisAddress string   `validate:"required,omitempty"`
}

func (a *Args) Strings() []string {
	var args []string
	args = append(args, "--lock-mode", a.LockMode)
	args = append(args, "--redis-addresses", strings.Join(a.RedisAddresses, ","))
	args = append(args, "--etcd-addresses", strings.Join(a.EtcdAddresses, ","))
	args = append(args, "--queue-redis-address", a.QueueRedisAddress)
	return args
}

func DefaultArgs() *Args {
	return &Args{
		LockMode:          "redlock",
		RedisAddresses:    []string{"127.0.0.1:6379"},
		EtcdAddresses:     []string{"127.0.0.1:2379"},
		QueueRedisAddress: "127.0.0.1:6379",
	}
}
