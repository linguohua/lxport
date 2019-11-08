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

// PairWSHandler process device websocket connection
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
		handlePairAdvice(c, uuid)
	case "req":
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

func handlePairRequest(c *websocket.Conn, uuid string, port uint16) {
	dev, ok := devices[uuid]
	if !ok {
		log.Println("handlePairRequest no device found with uuid:", uuid)
		return
	}

	pairUUID, err := gouuid.NewV4()
	if err != nil {
		log.Printf("handlePairRequest Something went wrong: %s", err)
		return
	}

	puuid := pairUUID.String()
	pair := newPair(puuid, dev, c)
	pairs[puuid] = pair

	defer func() {
		delete(pairs, puuid)
	}()

	pair.sendReq(port)

	select {
	case <-pair.pch:
	case <-time.After(5 * time.Second):
		log.Println("handlePairRequest, timeout")
		return
	}

	pair.loopMaster()
}

func handlePairResponse(c *websocket.Conn, uuid string) {
	pair, ok := pairs[uuid]
	if !ok {
		log.Println("handlePairResponse no pair found with uuid:", uuid)
		return
	}

	pair.onSlaveConneted(c)
	pair.pch <- true

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
