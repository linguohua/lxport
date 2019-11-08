package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	log "github.com/sirupsen/logrus"

	"lxport/endpointc"
	"lxport/wait"
)

var (
	lport  int
	rport  int
	uuid   string
	wsURL  string
	daemon = ""
)

func init() {
	flag.IntVar(&lport, "l", 8009, "specify the listen port")
	flag.IntVar(&rport, "r", 3389, "specify target port")
	flag.StringVar(&uuid, "u", "", "specify device uuid")
	flag.StringVar(&wsURL, "url", "", "specify web ssh path")
	flag.StringVar(&daemon, "d", "yes", "specify daemon mode")
}

// getVersion get version
func getVersion() string {
	return "0.1.0"
}

func main() {
	// only one thread
	runtime.GOMAXPROCS(1)

	version := flag.Bool("v", false, "show version")

	flag.Parse()

	if *version {
		fmt.Printf("%s\n", getVersion())
		os.Exit(0)
	}

	log.Println("try to start  lxport endpoint client, version:", getVersion())

	if uuid == "" {
		log.Fatal("please specify target device uuid")
	}

	if wsURL == "" {
		log.Fatal("please specify websocket URL")
	}

	params := &endpointc.Params{
		LocalPort:  uint16(lport),
		RemotePort: uint16(rport),
		UUID:       uuid,
		WsURL:      wsURL,
	}

	// start http server
	go endpointc.Run(params)
	log.Println("start lxport endpoint client ok!")

	if daemon == "yes" {
		wait.GetSignal()
	} else {
		wait.GetInput()
	}
	return
}
