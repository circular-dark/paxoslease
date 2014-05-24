package leaserpc

type RemoteLeaseNode interface {
	Prepare(args *Args, reply *Reply) error
	Accept(args *Args, reply *Reply) error
	RenewPrepare(args *Args, reply *Reply) error
	RenewAccept(args *Args, reply *Reply) error
}

type LeaseNode struct {
	RemoteLeaseNode
}

func Wrap(t RemoteLeaseNode) RemoteLeaseNode {
	return &LeaseNode{t}
}
