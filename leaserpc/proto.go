package leaserpc

type Status int

const (
	OK     Status = iota + 1 //The RPC was a success
	Reject                   //The RPC was a rejection
)

type Args struct {
	N int
}

type Reply struct {
	Status Status
}
