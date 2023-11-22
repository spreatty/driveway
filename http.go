package main

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func startHTTP() {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.Default()

	engine.StaticFile("/hud", "static/hud.html")
	engine.StaticFile("/favicon.ico", "static/favicon.ico")
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
	if err != nil {
		log.Println("Failed upgrading to websocket", err.Error())
		return
	}

	go func() {
		err := gate.Open(nil)
		message := GATE_CONNECT
		if err != nil {
			message = GATE_ERROR
		}
		conn.WriteMessage(websocket.TextMessage, []byte(message))
	}()

	if supportGarage {
		go func() {
			err := garage.Open(nil)
			message := GARAGE_CONNECT
			if err != nil {
				message = GARAGE_ERROR
			}
			conn.WriteMessage(websocket.TextMessage, []byte(message))
		}()
	}

	go func() {
		ticker := time.NewTicker(time.Second)
		var err error
		for range ticker.C {
			err = conn.WriteMessage(websocket.TextMessage, []byte(PING))
			if err != nil {
				// Disconnected or lost connection
				break
			}
		}
		log.Println("Ping failed", err.Error())
		ticker.Stop()
		gate.ScheduleClosing()
		if supportGarage {
			garage.ScheduleClosing()
		}
	}()

	go func() {
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
				res := gate.Press()
				conn.WriteMessage(websocket.TextMessage, []byte(GATE+":"+botResponse(res)))
			case message == GARAGE && supportGarage:
				res := garage.Press()
				conn.WriteMessage(websocket.TextMessage, []byte(GARAGE+":"+botResponse(res)))
			default:
				log.Println("Unexpected message", message)
			}
		}
	}()
}

func botResponse(res byte) string {
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
