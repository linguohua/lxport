package server

import (
	"lxport/server/tunpair"
	"net"
	"strings"
	"sync"
	"time"

	"encoding/binary"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"

	"net/http"
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: checkOrigin,
	} // use default options

	wsIndex = 0
	wsmap   = make(map[int]*wsholder)
)

func checkOrigin(_ *http.Request) bool {
	return true
}

type wsholder struct {
	id        int
	conn      *websocket.Conn
	writeLock sync.Mutex
	waitping  int
}

func newHolder(c *websocket.Conn) *wsholder {
	wsIndex++
	wsh := &wsholder{
		conn: c,
		id:   wsIndex,
	}

	// ping handle
	c.SetPingHandler(func(data string) error {
		wsh.writePong([]byte(data))
		return nil
	})

	// pong handler
	c.SetPongHandler(func(data string) error {
		wsh.onPong([]byte(data))
		return nil
	})

	return wsh
}

func (wsh *wsholder) write(msg []byte) error {
	wsh.writeLock.Lock()
	err := wsh.conn.WriteMessage(websocket.BinaryMessage, msg)
	wsh.writeLock.Unlock()

	return err
}

func (wsh *wsholder) keepalive() {
	if wsh.waitping > 3 {
		wsh.conn.Close()
		return
	}

	wsh.writeLock.Lock()
	now := time.Now().Unix()
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(now))
	wsh.conn.WriteMessage(websocket.PingMessage, b)
	wsh.writeLock.Unlock()

	wsh.waitping++
}

func (wsh *wsholder) writePong(msg []byte) {
	wsh.writeLock.Lock()
	wsh.conn.WriteMessage(websocket.PongMessage, msg)
	wsh.writeLock.Unlock()
}

func (wsh *wsholder) onPong(msg []byte) {
	// log.Println("wsh on pong")
	wsh.waitping = 0
}

// xportWSHandler handle xport websocket
func xportWSHandler(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	// ensoure websocket closed final
	defer c.Close()

	var query = r.URL.Query()
	var portStr = query.Get("port")
	if portStr == "" {
		log.Println("need port!")
		return
	}

	var target = query.Get("target")
	if target == "" {
		target = "127.0.0.1"
	}
	// only allow connect to localhost
	tcp, err := net.Dial("tcp", target+":"+portStr)
	if err != nil {
		log.Printf("dial to %s:%s failed: %v", target, portStr, err)
		return
	}

	wsh := newHolder(c)
	// save to map for keep-alive
	wsmap[wsh.id] = wsh

	defer func() {
		// ensoure tcp closed final
		tcp.Close()
		// delete from map
		delete(wsmap, wsh.id)
	}()

	// recv tcp message, and forward to websocket
	recvBuf := make([]byte, 4096)
	go func() {
		for {
			n, err := tcp.Read(recvBuf)
			if err != nil {
				log.Println("read from tcp failed:", err)
				c.Close()

				return
			}

			if n == 0 {
				log.Println("read from tcp got 0 bytes")
				c.Close()

				return
			}

			//log.Println("tcp recv message, len:", n)
			err = wsh.write(recvBuf[:n])
			if err != nil {
				log.Println("write all to websocket failed:", err)
				tcp.Close()
				return
			}
		}
	}()

	// recv websocket message and forward to tcp
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Println("websocket read failed:", err)
			tcp.Close()

			break
		}

		//log.Println("websocket recv message, len:", len(message))
		err = writeAll(message, tcp)
		if err != nil {
			log.Println("write all to tcp failed:", err)
			c.Close()

			break
		}
	}
}

// writeAll a function that ensure all bytes write out
// maybe it is unnecessary, if the underlying tcp connection has ensure that
func writeAll(buf []byte, nc net.Conn) error {
	wrote := 0
	l := len(buf)
	for {
		n, err := nc.Write(buf[wrote:])
		if err != nil {
			return err
		}

		if n == 0 {
			// should not happend
			break
		}

		wrote = wrote + n
		if wrote == l {
			break
		}
	}

	return nil
}

// keepalive send ping to all websocket
func keepalive() {
	for {
		time.Sleep(time.Second * 30)

		// first keepalive all xport/web-ssh websocket
		for _, v := range wsmap {
			v.keepalive()
		}

		// then keepalive pair websocket
		tunpair.Keepalive()
	}
}

// Params parameters
type Params struct {
	// server listen address
	ListenAddr string
	// xport http path
	XPortPath string
	// web ssh http path
	WebPath string
	// web ssh html/css file path
	WebDir string
	// pair http path
	PairPath string
}

// CreateHTTPServer start http server
func CreateHTTPServer(params *Params) {
	// start keepalive goroutine
	go keepalive()

	// xport
	http.HandleFunc(params.XPortPath, xportWSHandler)
	// pair
	http.HandleFunc(params.PairPath, tunpair.PairWSHandler)

	// web ssh
	if params.WebDir != "" && params.WebPath != "" {
		directory := params.WebDir // "/home/abc/webpack-starter/build"
		http.Handle("/", http.StripPrefix(strings.TrimRight(params.WebPath, "/"),
			http.FileServer(http.Dir(directory))))

		websocketPath := strings.TrimRight(params.WebPath, "/") + "/ws"
		http.HandleFunc(websocketPath, webSSHHandler)

		log.Printf("start with webssh support, websoket:%s, web path:%s, web dir:%s",
			websocketPath, params.WebPath, params.WebDir)
	} else {
		log.Warn("start without webssh support")
	}

	log.Printf("server listen at:%s, xportPath:%s, pair path:%s", params.ListenAddr,
		params.XPortPath, params.PairPath)
	log.Fatal(http.ListenAndServe(params.ListenAddr, nil))
}
