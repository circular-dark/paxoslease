package lease

import (
	"github.com/circulardark/paxoslease/leaserpc"
)

type LeaseNode interface {
	Prepare(args *leaserpc.Args, reply *leaserpc.Reply) error
	Accept(args *leaserpc.Args, reply *leaserpc.Reply) error
	RenewPrepare(args *leaserpc.Args, reply *leaserpc.Reply) error
	RenewAccept(args *leaserpc.Args, reply *leaserpc.Reply) error
	CheckMaster() bool
}
