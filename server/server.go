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

func xportWSHandler(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()

	var portStr = r.URL.Query().Get("port")
	if portStr == "" {
		log.Println("need port!")
		return
	}

	tcp, err := net.Dial("tcp", "127.0.0.1:"+portStr)
	if err != nil {
		log.Printf("dial to 127.0.0.1:%s failed: %v", portStr, err)
		return
	}

	wsIndex++
	wsh := &wsholder{
		conn: c,
		id:   wsIndex,
	}
	wsmap[wsh.id] = wsh

	defer tcp.Close()
	defer delete(wsmap, wsh.id)

	c.SetPingHandler(func(data string) error {
		wsh.writePong([]byte(data))
		return nil
	})

	c.SetPongHandler(func(data string) error {
		wsh.onPong([]byte(data))
		return nil
	})

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

func writeAll(buf []byte, nc net.Conn) error {
	wrote := 0
	l := len(buf)
	for {
		n, err := nc.Write(buf[wrote:])
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

func keepalive() {
	for {
		time.Sleep(time.Second * 30)

		for _, v := range wsmap {
			v.keepalive()
		}

		tunpair.Keepalive()
	}
}

// Params parameters
type Params struct {
	ListenAddr string
	XPortPath  string
	WebPath    string
	WebDir     string
	PairPath   string
}

// CreateHTTPServer start http server
func CreateHTTPServer(params *Params) {
	go keepalive()
	http.HandleFunc(params.XPortPath, xportWSHandler)

	// pair
	http.HandleFunc(params.PairPath, tunpair.PairWSHandler)

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

	log.Printf("server listen at:%s, xportPath:%s", params.ListenAddr, params.XPortPath)
	log.Fatal(http.ListenAndServe(params.ListenAddr, nil))
}
