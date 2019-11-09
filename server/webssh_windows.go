package server

import (
	"net/http"

	log "github.com/sirupsen/logrus"
)

// webSSHHandler windows not support ssh
func webSSHHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("windows not support web ssh")
}
