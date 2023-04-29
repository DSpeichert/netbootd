package main

import (
	"github.com/DSpeichert/netbootd/cmd"
)

//go:generate protoc proto/*.proto --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative

func main() {
	cmd.Execute()
}
