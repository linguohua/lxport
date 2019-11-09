package endpoints

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
)

var (
	// the device id
	deviceID string
	// websocket url, that is
	// base websocket url concatenated with device id
	wsURLRegister string
	// base websocket url
	wsURLBase string

	// map keep all current websocket
	// use for keep-alive
	wsholderMap = make(map[string]*wsholder)
)

// Params parameters
type Params struct {
	// device id
	UUID string
	// base websocket url
	WsURL string
}

// keepalive send ping to all websocket holder
func keepalive() {
	for {
		time.Sleep(time.Second * 30)

		for _, v := range wsholderMap {
			v.keepalive()
		}
	}
}

// Run run endpoint server and
// wait for server's command
func Run(params *Params) {
	deviceID = params.UUID
	wsURLRegister = fmt.Sprintf("%s?pt=dev&uuid=%s", params.WsURL, deviceID)
	wsURLBase = params.WsURL

	// keep-alive goroutine
	go keepalive()

	log.Printf("endpoint run, device uuid:%s", deviceID)
	cmdwsService()
}
