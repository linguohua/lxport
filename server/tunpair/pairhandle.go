// Package tunpair pair endpoint-c and endpoint-s
// endpoint-c(endpointc) run on client computer
// endpoint-s(endpoints) run on target computer
// client(eg. rdp:remote desktop) <----tcp-----> endpoint-c <-----ws--------> pair <-----ws-------> endpoint-s <----tcp----> target port
package tunpair

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	gouuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: checkOrigin,
	} // use default options

	devices = make(map[string]*Device)
	pairs   = make(map[string]*Pair)
)

func checkOrigin(_ *http.Request) bool {
	return true
}

// PairWSHandler handle pair request and response connection
func PairWSHandler(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("PairWSHandler upgrade:", err)
		return
	}
	defer c.Close()

	query := r.URL.Query()
	pairType := query.Get("pt")
	uuid := query.Get("uuid")

	switch pairType {
	case "dev":
		// device register, from endpoint-s endpoint server
		handlePairDevice(c, uuid)
	case "req":
		// port that endpoint-s will connect to
		portStr := query.Get("port")
		if portStr == "" {
			log.Println("PairWSHandler, need port")
			return
		}
		port, err := strconv.Atoi(portStr)
		if err != nil {
			log.Println("PairWSHandler, need port, Atoi failed:", err)
			return
		}

		handlePairRequest(c, uuid, uint16(port))
	case "resp":
		handlePairResponse(c, uuid)
	default:
		log.Println("PairWSHandler, unsupport pairtype:", pairType)
	}
}

// handlePairRequest endpoint-c client require create new pair to endpoint-s
func handlePairRequest(c *websocket.Conn, uuid string, port uint16) {
	// get target device
	dev, ok := devices[uuid]
	if !ok {
		log.Println("handlePairRequest no device found with uuid:", uuid)
		return
	}

	// generate a new pair uuid
	pairUUID, err := gouuid.NewV4()
	if err != nil {
		log.Printf("handlePairRequest Something went wrong: %s", err)
		return
	}

	puuid := pairUUID.String()
	// create a new pair object
	pair := newPair(puuid, dev, c)
	pairs[puuid] = pair

	defer func() {
		// ensure pair will be deleted final
		delete(pairs, puuid)
	}()

	// send pair creation request to target device
	pair.sendPairCreateReq(port)

	// wait the target device(endpoint-s) to reply or timeout
	select {
	case <-pair.pch:
	case <-time.After(5 * time.Second):
		log.Println("handlePairRequest, timeout")
		return
	}

	// read all master websocket message and forward to slave websocket
	pair.loopMaster()
}

// handlePairResponse endpoint-s response that the pair has created
func handlePairResponse(c *websocket.Conn, uuid string) {
	pair, ok := pairs[uuid]
	if !ok {
		log.Println("handlePairResponse no pair found with uuid:", uuid)
		return
	}

	// save slave connection
	pair.onSlaveConneted(c)
	// notify pair create request goroutine
	// that the pair has established
	pair.pch <- struct{}{}

	// read all slave websocket message and forward to master websocket
	pair.loopSlave()
}

// Keepalive do keepalive and check
func Keepalive() {
	for _, v := range devices {
		v.keepalive()
	}

	for _, v := range pairs {
		v.keepalive()
	}
}
