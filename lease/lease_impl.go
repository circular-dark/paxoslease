package lease

import (
	"fmt"
	"github.com/circulardark/paxoslease/leaserpc"
	"net/rpc"
	"sync"
	"time"
)

type Node struct {
	AddrPort string
	NodeID   int
}

type leaseNode struct {
	addrPort       string
	nodeID         int
	numNodes       int
	peers          []Node //including itself
	nextBallot     int
	leaseMutex     sync.Mutex
	isMaster       bool
	masterLeaseLen int // remaining master lease length period
	renewLeaseLen  int // remaining renew master lease length period
	acceptLeaseLen int // remaining accept lease length period
	Nh             int
	Na             int
}

const (
	PERIOD_LEN  = 1000 // number of milliseconds in a period
	LEASE_LEN   = 5    // number of periods in a lease length
	REFRESH_LEN = 2    // number of remaining master periods when trying to renew
)

var (
	Nodes = [7]Node{
		Node{"127.0.0.1:54322", 0},
		Node{"127.0.0.1:54323", 1},
		Node{"127.0.0.1:54324", 2},
		Node{"127.0.0.1:54325", 3},
		Node{"127.0.0.1:54326", 4},
		Node{"127.0.0.1:54327", 5},
		Node{"127.0.0.1:54328", 6},
	}
)

//Current setting: all settings are static
func NewLeaseNode(nodeID int, numNodes int) (LeaseNode, error) {
	node := &leaseNode{
		addrPort:       Nodes[nodeID].AddrPort,
		nodeID:         nodeID,
		numNodes:       numNodes,
		isMaster:       false,
		nextBallot:     nodeID,
		masterLeaseLen: 0,
		renewLeaseLen:  0,
		acceptLeaseLen: 0,
		Nh:             0,
		Na:             0,
		peers:          make([]Node, numNodes),
	}

	for i := 0; i < numNodes; i++ {
		node.peers[i].AddrPort = Nodes[i].AddrPort
		node.peers[i].NodeID = Nodes[i].NodeID
	}

	if err := rpc.RegisterName("LeaseNode", leaserpc.Wrap(node)); err != nil {
		return nil, err
	}

	go node.leaseManager()

	return node, nil
}

func (ln *leaseNode) Prepare(args *leaserpc.Args, reply *leaserpc.Reply) error {
	ln.leaseMutex.Lock()
	if args.N < ln.Nh {
		reply.Status = leaserpc.Reject
	} else {
		ln.Nh = args.N
		if ln.masterLeaseLen > 0 || ln.acceptLeaseLen > 0 {
			reply.Status = leaserpc.Reject
		} else {
			reply.Status = leaserpc.OK
		}
	}
	ln.leaseMutex.Unlock()
	return nil
}

func (ln *leaseNode) Accept(args *leaserpc.Args, reply *leaserpc.Reply) error {
	ln.leaseMutex.Lock()
	if args.N < ln.Nh {
		reply.Status = leaserpc.Reject
	} else {
		ln.Nh = args.N
		ln.Na = args.N
		ln.acceptLeaseLen = LEASE_LEN
		reply.Status = leaserpc.OK
	}
	ln.leaseMutex.Unlock()
	return nil
}

func (ln *leaseNode) dofunction(args *leaserpc.Args, reply *leaserpc.Reply, funcname string) error {
	replychan := make(chan *leaserpc.Reply, len(ln.peers))

	for _, n := range ln.peers {
		go func(peernode Node) {
			if peernode.AddrPort == ln.addrPort {
				r := leaserpc.Reply{}
				switch {
				case funcname == "LeaseNode.Prepare":
					ln.Prepare(args, &r)
				case funcname == "LeaseNode.Accept":
					ln.Accept(args, &r)
				case funcname == "LeaseNode.RenewPrepare":
					ln.RenewPrepare(args, &r)
				case funcname == "LeaseNode.RenewAccept":
					ln.RenewAccept(args, &r)
				}
				replychan <- &r
			} else {
				r := leaserpc.Reply{}
				peer, err := rpc.DialHTTP("tcp", peernode.AddrPort)
				if err != nil {
					r.Status = leaserpc.Reject
					replychan <- &r
					return
				}
				prepareCall := peer.Go(funcname, args, &r, nil)
				select {
				case _, _ = <-prepareCall.Done:
					replychan <- &r
				case _ = <-time.After(time.Second):
					r.Status = leaserpc.Reject
					replychan <- &r
				}
				peer.Close()
			}
		}(n)
	}

	numOK, numRej := 0, 0
	for num := 0; num < len(ln.peers); num++ {
		r, _ := <-replychan
		if r.Status == leaserpc.OK {
			numOK++
		} else {
			numRej++
		}
	}

	if numOK > ln.numNodes/2 {
		reply.Status = leaserpc.OK
		return nil
	} else {
		reply.Status = leaserpc.Reject
		return nil
	}
}

func (ln *leaseNode) DoPrepare(args *leaserpc.Args, reply *leaserpc.Reply) error {
	return ln.dofunction(args, reply, "LeaseNode.Prepare")
}

func (ln *leaseNode) DoAccept(args *leaserpc.Args, reply *leaserpc.Reply) error {
	return ln.dofunction(args, reply, "LeaseNode.Accept")
}

func (ln *leaseNode) RenewPrepare(args *leaserpc.Args, reply *leaserpc.Reply) error {
	ln.leaseMutex.Lock()
	if ln.Nh < args.N {
		ln.Nh = args.N
	}
	if ln.Na == args.N || (ln.acceptLeaseLen == 0 && ln.masterLeaseLen == 0) {
		reply.Status = leaserpc.OK
	} else {
		reply.Status = leaserpc.Reject
	}
	ln.leaseMutex.Unlock()
	return nil
}

func (ln *leaseNode) RenewAccept(args *leaserpc.Args, reply *leaserpc.Reply) error {
	ln.leaseMutex.Lock()
	if ln.Nh < args.N {
		ln.Nh = args.N
	}
	if ln.Na == args.N || (ln.acceptLeaseLen == 0 && ln.masterLeaseLen == 0) {
		ln.acceptLeaseLen = LEASE_LEN
		reply.Status = leaserpc.OK
	} else {
		reply.Status = leaserpc.Reject
	}
	ln.leaseMutex.Unlock()
	return nil
}

func (ln *leaseNode) DoRenewPrepare(args *leaserpc.Args, reply *leaserpc.Reply) error {
	return ln.dofunction(args, reply, "LeaseNode.RenewPrepare")
}

func (ln *leaseNode) DoRenewAccept(args *leaserpc.Args, reply *leaserpc.Reply) error {
	return ln.dofunction(args, reply, "LeaseNode.RenewAccept")
}

func (ln *leaseNode) renewLease() {
	// Prepare
	ln.leaseMutex.Lock()
	prepareArgs := leaserpc.Args{
		N: ln.Na,
	}
	ln.leaseMutex.Unlock()
	prepareReply := leaserpc.Reply{}

	ln.DoRenewPrepare(&prepareArgs, &prepareReply)
	if prepareReply.Status == leaserpc.Reject {
		return
	}

	// Accept
	ln.leaseMutex.Lock()
	ln.renewLeaseLen = LEASE_LEN - 2 // for safe now, shorten the length a bit
	ln.leaseMutex.Unlock()
	acceptArgs := leaserpc.Args{
		N: prepareArgs.N,
	}
	acceptReply := leaserpc.Reply{}

	ln.DoRenewAccept(&acceptArgs, &acceptReply)
	if acceptReply.Status == leaserpc.Reject {
		return
	}

	ln.leaseMutex.Lock()
	if ln.renewLeaseLen > 0 {
		ln.isMaster = true
		ln.masterLeaseLen = ln.renewLeaseLen
		fmt.Printf("node %d IS AGAIN THE MASTER NOW!\n", ln.nodeID)
	}
	ln.Na = acceptArgs.N
	ln.leaseMutex.Unlock()
}

func (ln *leaseNode) getLease() {
	// Prepare
	ln.leaseMutex.Lock()
	for ln.nextBallot <= ln.Nh {
		ln.nextBallot += ln.numNodes
	}
	prepareArgs := leaserpc.Args{
		N: ln.nextBallot,
	}
	ln.leaseMutex.Unlock()
	prepareReply := leaserpc.Reply{}

	ln.DoPrepare(&prepareArgs, &prepareReply)
	if prepareReply.Status == leaserpc.Reject {
		return
	}

	// Accept
	ln.leaseMutex.Lock()
	ln.masterLeaseLen = LEASE_LEN - 2 // for safe now, shorten the length a bit
	ln.leaseMutex.Unlock()
	acceptArgs := leaserpc.Args{
		N: prepareArgs.N,
	}
	acceptReply := leaserpc.Reply{}

	ln.DoAccept(&acceptArgs, &acceptReply)
	if acceptReply.Status == leaserpc.Reject {
		return
	}

	ln.leaseMutex.Lock()
	if ln.masterLeaseLen > 0 {
		ln.isMaster = true
		fmt.Printf("node %d IS THE MASTER NOW!\n", ln.nodeID)
	}
	ln.Na = acceptArgs.N
	ln.leaseMutex.Unlock()
}

func (ln *leaseNode) CheckMaster() bool {
	ln.leaseMutex.Lock()
	res := (ln.isMaster && ln.masterLeaseLen > 0)
	ln.leaseMutex.Unlock()
	return res
}

func (ln *leaseNode) leaseManager() {
	t := time.NewTicker(PERIOD_LEN * time.Millisecond)
	for {
		select {
		case <-t.C:
			willget := false
			willrenew := false
			ln.leaseMutex.Lock()
			if ln.renewLeaseLen > 0 {
				ln.renewLeaseLen--
			}
			if ln.masterLeaseLen > 0 {
				ln.masterLeaseLen--
			}
			if ln.acceptLeaseLen > 0 {
				ln.acceptLeaseLen--
			}
			if ln.isMaster {
				if ln.masterLeaseLen == 0 {
					ln.isMaster = false
					fmt.Printf("node %d IS  NOT  THE MASTER NOW!\n", ln.nodeID)
				} else if ln.masterLeaseLen < REFRESH_LEN {
					willrenew = true
				}
			}
			if ln.acceptLeaseLen == 0 {
				willget = true
			}
			ln.leaseMutex.Unlock()
			if willrenew {
				ln.renewLease()
			} else if willget {
				ln.getLease()
			}
		}
	}
}
