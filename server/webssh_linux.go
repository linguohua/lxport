package server

import (
	"encoding/json"
	"io"
	"net/http"
	"os/exec"

	"github.com/creack/pty"
	log "github.com/sirupsen/logrus"
)

// SizeInfo ssh json message
type SizeInfo struct {
	Rows uint16 `json:"rows"`
	Cols uint16 `json:"cols"`
}

// webSSHHandler handle web-ssh websocket(from web-browser) connection
func webSSHHandler(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	// ensure websocket will be closed final
	defer c.Close()

	wsh := newHolder(c)
	// save to map for keepalive
	wsmap[wsh.id] = wsh

	defer delete(wsmap, wsh.id)

	// Create arbitrary command.
	cmd := exec.Command("bash")

	// Start the command with a pty.
	ptmx, err := pty.Start(cmd)
	if err != nil {
		log.Println("pty Start failed:", err)
		return
	}

	defer func() {
		// ensure cmd will exit final
		if cmd.Process != nil {
			err = cmd.Process.Kill()
			if err != nil {
				log.Println("cmd.Process.Kill error:", err)
			}
		}

		// wait cmd exit
		err = cmd.Wait()
		if err != nil {
			log.Println("cmd.Wait error:", err)
		}

		// close ptmx
		err = ptmx.Close()
		if err != nil {
			log.Println("ptmx.Close:", err)
		} else {
			log.Println("ptmx closed!")
		}
	}()

	// bridge ptmx message to websocket
	go pipe2WS(ptmx, wsh)

	// read websocket message and forward to ptmx
	loop := true
	for loop {
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Println("websocket read failed:", err)
			break
		}

		op := message[0]
		//log.Printf("websocket recv message, len:%d, op:%d ", len(message), op)

		switch op {
		case 0:
			// term command
			err = ws2Pipe(message[1:], ptmx)
			if err != nil {
				log.Println("write all to ptmx failed:", err)
				c.Close()

				loop = false
			}
		case 1:
			// ping
			message[0] = 2
			wsh.write(message)
		case 2:
			// pong
			break
		case 3:
			// resize
			sz := &SizeInfo{}
			err = json.Unmarshal(message[1:], sz)
			if err == nil {
				ws := &pty.Winsize{
					Rows: sz.Rows,
					Cols: sz.Cols,
				}
				pty.Setsize(ptmx, ws)
			} else {
				log.Println("SetSize failed:", err)
			}
		}
	}

	log.Println("webSSHHandler completed")
}

// ws2Pipe forward websocket's messsage to ptmx
func ws2Pipe(buf []byte, writer io.WriteCloser) error {
	wrote := 0
	l := len(buf)
	for {
		n, err := writer.Write(buf[wrote:])
		if err != nil {
			return err
		}

		wrote = wrote + n
		if wrote == l {
			break
		}
	}

	return nil
}

// pipe2WS bridge ptmx message to websocket
func pipe2WS(pipe io.ReadCloser, c *wsholder) {
	buf := make([]byte, 4096)
	for {
		n, err := pipe.Read(buf)
		if err != nil {
			log.Println("pipeWS, pipe read failed:", err)
			break
		}

		if n < 1 {
			break
		}

		// one byte message cmd type
		b := make([]byte, n+1)
		b[0] = 0 // always is 0, means ptmx data
		copy(b[1:], buf[0:n])
		c.write(b)
	}

	log.Println("pipe2WS completed")
}
