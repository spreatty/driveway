package main

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func startHTTP() {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.Default()

	engine.StaticFile("/hud", "static/hud.html")
	engine.StaticFile("/gate", "static/gate.html")
	engine.StaticFile("/favicon.ico", "static/favicon.ico")
	engine.StaticFile("/manifest.json", "static/manifest.json")
	engine.Static("/static", "static")

	engine.GET("/"+Config.Server.AuthToken+"/ws", func(c *gin.Context) {
		wshandler(c.Writer, c.Request, true)
	})
	engine.GET("/"+Config.Server.AuthToken+"/mono", func(c *gin.Context) {
		wshandler(c.Writer, c.Request, false)
	})

	var err error
	if Config.Server.UseTLS {
		err = engine.RunTLS(Config.Server.Address, Config.Server.TLSCert, Config.Server.TLSKey)
	} else {
		err = engine.Run(Config.Server.Address)
	}
	if err != nil {
		log.Fatalln("Failed to start HTTP server", err)
	}
}

const PING = "ping"
const GATE_CONNECT = "gateconnect"
const GATE_ERROR = "gateerror"
const GARAGE_CONNECT = "garageconnect"
const GARAGE_ERROR = "garageerror"
const GATE = "gate"
const GARAGE = "garage"

var wsupgrader = websocket.Upgrader{}
var gate = Bot{Address: addrMap[Config.Bots.Gate]}
var garage = Bot{Address: addrMap[Config.Bots.Garage]}

func wshandler(writer http.ResponseWriter, req *http.Request, supportGarage bool) {
	conn, err := wsupgrader.Upgrade(writer, req, nil)
	var wsmu sync.Mutex
	if err != nil {
		log.Println("Failed upgrading to websocket", err.Error())
		return
	}

	supportGate := true
	if supportGate {
		go func() {
			err := gate.Open(nil)
			message := GATE_CONNECT
			if err != nil {
				message = GATE_ERROR
			}
			wsmu.Lock()
			conn.WriteMessage(websocket.TextMessage, []byte(message))
			wsmu.Unlock()
		}()
	}

	//supportGarage = false
	if supportGarage {
		go func() {
			err := garage.Open(nil)
			message := GARAGE_CONNECT
			if err != nil {
				message = GARAGE_ERROR
			}
			wsmu.Lock()
			conn.WriteMessage(websocket.TextMessage, []byte(message))
			wsmu.Unlock()
		}()
	}

	go func() {
		ticker := time.NewTicker(time.Second)
		var err error
		for range ticker.C {
			wsmu.Lock()
			err = conn.WriteMessage(websocket.TextMessage, []byte(PING))
			wsmu.Unlock()
			if err != nil {
				// Disconnected or lost connection
				break
			}
		}
		log.Println("Ping failed", err.Error())
		ticker.Stop()
		if supportGate {
			go gate.Abandon()
		}
		if supportGarage {
			go garage.Abandon()
		}
	}()

	for {
		messageType, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("WebSocket error", err.Error())
			break
		}
		if messageType != websocket.TextMessage {
			log.Println("Skipped non-text message")
			continue
		}

		message := string(msg)
		switch {
		case message == GATE:
			_, err = gate.Press()
			answer := "ok"
			if err != nil {
				answer = "error"
			}
			log.Println("Gate says \"" + answer + "\"")
			wsmu.Lock()
			conn.WriteMessage(websocket.TextMessage, []byte(GATE+":"+answer))
			wsmu.Unlock()
		case message == GARAGE && supportGarage:
			_, err = garage.Press()
			answer := "ok"
			if err != nil {
				answer = "error"
			}
			log.Println("Garage says \"" + answer + "\"")
			wsmu.Lock()
			conn.WriteMessage(websocket.TextMessage, []byte(GARAGE+":"+answer))
			wsmu.Unlock()
		default:
			log.Println("Unexpected message", message)
		}
	}
}

func botAnswer(res byte) string {
	switch res {
	case StatusOK:
		return "ok"
	case StatusBusy:
		return "busy"
	case StatusLowBattery:
		return "low"
	default:
		return "error"
	}
}
