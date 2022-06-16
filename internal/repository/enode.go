/*
Package repository implements repository for handling fast and efficient access to data required
by the resolvers of the API server.

Internally it utilizes RPC to access Opera/Opera full node for blockchain interaction. Mongo database
for fast, robust and scalable off-chain data storage, especially for aggregated and pre-calculated data mining
results. BigCache for in-memory object storage to speed up loading of frequently accessed entities.
*/
package repository

import (
	"fantom-api-graphql/internal/repository/db"
	"fantom-api-graphql/internal/types"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"time"
)

// NetworkNode returns instance of Opera network node record by its ID.
func (p *proxy) NetworkNode(nid enode.ID) (*types.OperaNode, error) {
	return p.db.NetworkNode(nid)
}

// StoreNetworkNode stores the given Opera node record in the persistent database.
func (p *proxy) StoreNetworkNode(node *types.OperaNode) error {
	return p.db.StoreNetworkNode(node)
}

// IsNetworkNodeKnown checks if the given network node is already registered in the persistent database.
func (p *proxy) IsNetworkNodeKnown(id enode.ID) bool {
	return p.db.IsNetworkNodeKnown(id)
}

// NetworkNodeConfirmCheck confirms successful check of the given Opera network node.
func (p *proxy) NetworkNodeConfirmCheck(node *enode.Node) (bool, error) {
	err := p.db.NetworkNodeConfirmCheck(node.ID())
	if err == nil {
		return false, nil
	}

	// other error
	if err != db.ErrUnknownNetworkNode {
		return false, err
	}

	// make new node
	now := time.Now().UTC()
	return true, p.db.StoreNetworkNode(&types.OperaNode{
		Node:              *node,
		Score:             1,
		CheckFailureCount: 0,
		FirstResponse:     now,
		LastResponse:      now,
		LastCheck:         now,
	})
}

// NetworkNodeFailCheck registers failed check of the given Opera network node.
func (p *proxy) NetworkNodeFailCheck(node *enode.Node) error {
	return p.db.NetworkNodeFailCheck(node.ID())
}

// NetworkNodeUpdateBatch provides a list of Opera network node addresses most suitable for status update
// based on the registered time of the latest check.
func (p *proxy) NetworkNodeUpdateBatch() ([]*enode.Node, error) {
	return p.db.NetworkNodeUpdateBatch()
}

// NetworkNodeBootstrapSet provides a set of known nodes to be co-used to bootstrap new search.
func (p *proxy) NetworkNodeBootstrapSet() []*enode.Node {
	return p.db.NetworkNodeBootstrapSet()
}