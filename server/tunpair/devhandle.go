package tunpair

import (
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

func handlePairAdvice(c *websocket.Conn, uuid string) {
	if uuid == "" {
		log.Println("need uuid provided")
		return
	}

	peerAddr := c.RemoteAddr()
	log.Println("accept device websocket from:", peerAddr)
	defer c.Close()

	// wait old xdevice to exit
	old, ok := devices[uuid]
	if ok {
		old.close()
		old.wg.Wait()
		log.Println("wait old device exit ok:", uuid)
	}

	new := newDevice(uuid, c)
	_, ok = devices[uuid]
	if ok {
		log.Println("try to add device conflict")
		return
	}

	devices[uuid] = new
	new.wg.Add(1)
	defer func() {
		delete(devices, uuid)
		new.wg.Done()
	}()

	new.loopMsg()

}
