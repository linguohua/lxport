package server

import (
	"net/http"

	log "github.com/sirupsen/logrus"
)

func webSSHHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("windows not support web ssh")
}
