package endpoints

import (
	"fmt"

	log "github.com/sirupsen/logrus"
)

var (
	deviceID string
	wsURL    string
	wsURLRaw string
)

// Params parameters
type Params struct {
	UUID  string
	WsURL string
}

// Run run endpoint
func Run(params *Params) {
	deviceID = params.UUID
	wsURL = fmt.Sprintf("%s?pt=dev&uuid=%s", params.WsURL, deviceID)
	wsURLRaw = params.WsURL

	log.Printf("endpoint run, device uuid:%s", deviceID)
	cmdwsService()
}
