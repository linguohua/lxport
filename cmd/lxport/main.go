package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	log "github.com/sirupsen/logrus"

	"lxport/server"
	"lxport/wait"
)

var (
	listenAddr = ""
	xportPath  = ""
	webSSHPath = ""
	webDir     = ""
	daemon     = ""
	pairPath   = ""
)

func init() {
	flag.StringVar(&listenAddr, "l", "127.0.0.1:8010", "specify the listen address")
	flag.StringVar(&xportPath, "p", "/xport", "specify websocket path")
	flag.StringVar(&webSSHPath, "wp", "/webssh/", "specify web ssh path")
	flag.StringVar(&pairPath, "pp", "/pair", "specify web ssh path")
	flag.StringVar(&webDir, "wd", "", "specify web dir")
	flag.StringVar(&daemon, "d", "yes", "specify daemon mode")
}

// getVersion get version
func getVersion() string {
	return "0.1.1"
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

	log.Println("try to start  lxport server, version:", getVersion())

	if webDir == "" {
		log.Println("webDir not provided, will not support webssh")
	}

	params := &server.Params{
		ListenAddr: listenAddr,
		XPortPath:  xportPath,
		WebPath:    webSSHPath,
		WebDir:     webDir,
		PairPath:   pairPath,
	}

	// start http server
	go server.CreateHTTPServer(params)
	log.Println("start lxport server ok!")

	if daemon == "yes" {
		wait.GetSignal()
	} else {
		wait.GetInput()
	}
	return
}
