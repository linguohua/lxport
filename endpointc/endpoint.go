package endpointc

import (
	"fmt"

	log "github.com/sirupsen/logrus"
)

var (
	// local listen tcp port
	localPort uint16 //  = 8009
	// remote port, that endpoint server
	// should connect to
	remotePort uint16 // = 3389
	// device uuid
	deviceID string
	// base websocket url
	wsURL string
)

// Params parameters
type Params struct {
	// local listen tcp port
	LocalPort uint16
	// remote port, that endpoint server
	// should connect to
	RemotePort uint16
	// device uuid
	UUID string
	// base websocket url
	WsURL string
}

// Run run endpoint client and
// wait client to connect
func Run(params *Params) {
	localPort = params.LocalPort
	remotePort = params.RemotePort
	deviceID = params.UUID
	wsURL = fmt.Sprintf("%s?pt=req&uuid=%s&port=%d", params.WsURL, deviceID, remotePort)

	log.Printf("endpoint run, local port:%d, target port:%d, device uuid:%s", localPort, remotePort, deviceID)
	startTCPListener(localPort)
}
