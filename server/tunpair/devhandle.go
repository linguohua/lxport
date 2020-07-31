package tunpair

import (
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

// handlePairDevice handle pair-able device register
func handlePairDevice(c *websocket.Conn, uuid string) {
	if uuid == "" {
		log.Println("handlePairDevice need uuid provided")
		return
	}

	peerAddr := c.RemoteAddr()
	log.Println("handlePairDevice accept device websocket from:", peerAddr)
	defer c.Close()

	// if we have old websocket connection of this device, wait it to exit
	old, ok := devices[uuid]
	if ok {
		old.close()
		// wait
		old.wg.Wait()
		log.Println("handlePairDevice wait old device exit ok:", uuid)
	}

	// create new device and add to devices map
	new := newDevice(uuid, c)
	_, ok = devices[uuid]
	if ok {
		log.Println("handlePairDevice try to add device conflict")
		return
	}

	devices[uuid] = new
	new.wg.Add(1)
	defer func() {
		// remove from devices map
		delete(devices, uuid)
		new.wg.Done()
	}()

	// read device's websocket message
	new.loopMsg()
}
