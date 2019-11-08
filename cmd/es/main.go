package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	log "github.com/sirupsen/logrus"

	"lxport/endpoints"
	"lxport/wait"
)

var (
	uuid   string
	wsURL  string
	daemon = ""
)

func init() {
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

	log.Println("try to start  lxport endpoint server, version:", getVersion())

	if uuid == "" {
		log.Fatal("please specify device uuid")
	}

	if wsURL == "" {
		log.Fatal("please specify websocket URL")
	}

	params := &endpoints.Params{
		UUID:  uuid,
		WsURL: wsURL,
	}

	// start http server
	go endpoints.Run(params)
	log.Println("start lxport endpoint server ok!")

	if daemon == "yes" {
		wait.GetSignal()
	} else {
		wait.GetInput()
	}
	return
}
