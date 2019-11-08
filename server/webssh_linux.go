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

func webSSHHandler(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()

	wsIndex++
	wsh := &wsholder{
		conn: c,
		id:   wsIndex,
	}
	wsmap[wsh.id] = wsh

	defer delete(wsmap, wsh.id)

	c.SetPingHandler(func(data string) error {
		wsh.writePong([]byte(data))
		return nil
	})

	c.SetPongHandler(func(data string) error {
		wsh.onPong([]byte(data))
		return nil
	})

	// Create arbitrary command.
	cmd := exec.Command("bash")

	// Start the command with a pty.
	ptmx, err := pty.Start(cmd)
	if err != nil {
		log.Println("pty Start failed:", err)
		return
	}

	defer func() {
		if cmd.Process != nil {
			err = cmd.Process.Kill()
			if err != nil {
				log.Println("cmd.Process.Kill error:", err)
			}
		}

		err = cmd.Wait()
		if err != nil {
			log.Println("cmd.Wait error:", err)
		}

		err = ptmx.Close()
		if err != nil {
			log.Println("ptmx.Close:", err)
		} else {
			log.Println("ptmx closed!")
		}
	}()

	go pipe2WS(ptmx, wsh)

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

		b := make([]byte, n+1)
		b[0] = 0
		copy(b[1:], buf[0:n])
		c.write(b)
	}

	log.Println("pipe2WS completed")
}
