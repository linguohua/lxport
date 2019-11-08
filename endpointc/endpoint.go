package endpointc

import (
	"fmt"

	log "github.com/sirupsen/logrus"
)

var (
	localPort  uint16 //  = 8009
	remotePort uint16 // = 3389
	deviceID   string
	wsURL      string
)

// Params parameters
type Params struct {
	LocalPort  uint16
	RemotePort uint16
	UUID       string
	WsURL      string
}

// Run run endpoint
func Run(params *Params) {
	localPort = params.LocalPort
	remotePort = params.RemotePort
	deviceID = params.UUID
	wsURL = fmt.Sprintf("%s?pt=req&uuid=%s&port=%d", params.WsURL, deviceID, remotePort)

	log.Printf("endpoint run, local port:%d, target port:%d, device uuid:%s", localPort, remotePort, deviceID)
	startTCPListener(localPort)
}
