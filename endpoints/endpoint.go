package endpoints

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
)

var (
	deviceID string
	wsURL    string
	wsURLRaw string

	wsholderMap = make(map[string]*wsholder)
)

// Params parameters
type Params struct {
	UUID  string
	WsURL string
}

func keepalive() {
	for {
		time.Sleep(time.Second * 30)

		for _, v := range wsholderMap {
			v.keepalive()
		}

	}
}

// Run run endpoint
func Run(params *Params) {
	deviceID = params.UUID
	wsURL = fmt.Sprintf("%s?pt=dev&uuid=%s", params.WsURL, deviceID)
	wsURLRaw = params.WsURL

	go keepalive()

	log.Printf("endpoint run, device uuid:%s", deviceID)
	cmdwsService()
}
