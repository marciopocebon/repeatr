package cassandra_mem

import (
	"sync"

	"go.polydawn.net/repeatr/rsrch/model/cassandra"
	"go.polydawn.net/repeatr/rsrch/model/catalog"
	"go.polydawn.net/repeatr/rsrch/model/formula"
)

type Base struct {
	mutex       sync.Mutex
	commissions map[formula.CommissionID]*formula.Commission
	catalogs    map[catalog.ID]*catalog.Book
	formulas    map[formula.Stage2ID]*formula.Stage2
	results     map[formula.Stage3ID]*formula.Stage3

	commissionObservers []chan<- formula.CommissionID
	catalogObservers    []chan<- catalog.ID
}

func New() cassandra.Cassandra {
	return &Base{
		commissions: make(map[formula.CommissionID]*formula.Commission),
		catalogs:    make(map[catalog.ID]*catalog.Book),
		formulas:    make(map[formula.Stage2ID]*formula.Stage2),
		results:     make(map[formula.Stage3ID]*formula.Stage3),
	}
}
