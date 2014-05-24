package main

import (
	"fmt"
	"github.com/circulardark/paxoslease/lease"
	"net"
	"net/http"
	"net/rpc"
)

const (
	nid      = 0
	numNodes = 5
)

func main() {
	fmt.Printf("node %d starts\n", nid)
	_, err := lease.NewLeaseNode(nid, numNodes)
	if err != nil {
		fmt.Println("Cannot start node.\n")
		fmt.Println(err)
		return
	}
	listener, err := net.Listen("tcp", lease.Nodes[nid].AddrPort)
	if err != nil {
		fmt.Printf("node %d cannot listen to port:%s\n", nid, err)
		return
	}
	rpc.HandleHTTP()
	http.Serve(listener, nil)
}
