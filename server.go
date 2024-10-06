package main

import (
	"github.com/go-redis/redis"
	"github.com/gorilla/websocket"
	"github.com/labstack/gommon/log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

var redisClient = redis.NewClient(&redis.Options{
	Addr:     os.Getenv("redis_url"),
	Password: "",
	DB:       0,
})
var clients = make(map[string]*websocket.Conn)
var upgrade = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func main() {
	e := echo.New()
	e.GET("/chat", IdHandler)
	e.GET("/chat/ws", socketHandler)
	go channelHandler()
	e.Logger.Fatal(e.Start(":8080"))
}

func IdHandler(c echo.Context) error {
	cookie := new(http.Cookie)
	cookie.Name = "rakao_id"
	cookie.Value = uuid.New().String()
	cookie.Expires = time.Now().Add(time.Hour)
	cookie.Path = "/"
	cookie.HttpOnly = true
	c.SetCookie(cookie)
	return c.NoContent(http.StatusNoContent)
}

func socketHandler(c echo.Context) error {
	id, err := c.Cookie("rakao_id")
	if err != nil {
		return c.NoContent(http.StatusUnauthorized)
	}

	connection, err := upgrade.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		closeConnection(connection)
		return c.NoContent(http.StatusInternalServerError)
	}

	if partnerId := redisClient.Get(id.Value).Val(); partnerId != "" {
		go chatHandler(connection, id.Value, partnerId)
		return nil
	}

	clients[id.Value] = connection
	result, err := redisClient.RPop("waiting").Result()

	if result != "" {
		redisClient.Set(id.Value, result, time.Hour)
		redisClient.Set(result, id.Value, time.Hour)
		go chatHandler(connection, id.Value, result)
		return nil
	}

	redisClient.LPush("waiting", id.Value)
	timeout := time.After(3 * time.Minute)
	ticker := time.Tick(time.Second)
	for {
		select {
		case <-timeout:
			delete(clients, id.Value)
			closeConnection(connection)
			return nil
		case <-ticker:
			if val := redisClient.Get(id.Value).Val(); val != "" {
				go chatHandler(connection, id.Value, val)
				return nil
			}
		}
	}
}

func chatHandler(conn *websocket.Conn, id string, partnerId string) {
	writeMessage(conn, "connected: "+partnerId)
	for {
		messageType, msg, err := conn.ReadMessage()
		if err != nil {
			delete(clients, id)
			redisClient.Del(id)
			redisClient.Publish("rakao-exit", partnerId)
			closeConnection(conn)
			return
		}
		if messageType != websocket.TextMessage {
			continue
		}
		redisClient.Publish("rakao-chat", partnerId+":"+string(msg))
	}
}

func channelHandler() {
	subscriber := redisClient.Subscribe("rakao-chat", "rakao-exit")
	defer closeSubscriber(subscriber)
	for {
		msg, err := subscriber.ReceiveMessage()
		if err != nil {
			log.Errorf("channel error: %v", err)
		}
		switch msg.Channel {
		case "rakao-chat":
			idx := strings.Index(msg.Payload, ":")
			id := msg.Payload[:idx]
			if conn, ok := clients[id]; ok {
				writeMessage(conn, msg.Payload[idx+1:])
			}
		case "rakao-exit":
			id := msg.Payload
			if conn, ok := clients[id]; ok {
				delete(clients, id)
				redisClient.Del(id)
				closeConnection(conn)
			}
		}
	}
}

func closeConnection(conn *websocket.Conn) {
	if err := conn.Close(); err != nil {
		log.Errorf("error: %v", err)
	}
}

func closeSubscriber(sub *redis.PubSub) {
	if err := sub.Close(); err != nil {
		log.Errorf("error: %v", err)
	}
}

func writeMessage(conn *websocket.Conn, msg string) {
	if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
		log.Errorf("error: %v", err)
	}
}
