package main

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/olahol/melody"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type Message struct {
	Id        bson.ObjectId `json:"id" bson:"_id,omitempty"`
	User      string        `json:"user" bson:"user"`
	Text      string        `json:"text" bson:"text"`
	Ip        string        `json:"ip" bson:"ip"`
	Timestamp time.Time     `json:"timestamp,string" bson:"timestamp"`
}

func main() {
	r := gin.Default()
	m := melody.New()
	m.Upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}

	r.LoadHTMLGlob("html/*")

	session, err := mgo.Dial("localhost")
	if err != nil {
		panic(err)
	}
	defer session.Close()

	db := session.DB("shoutdb").C("shouts")

	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{})
	})

	r.GET("ws", func(c *gin.Context) {
		m.HandleRequest(c.Writer, c.Request)
	})

	r.GET("api/messages", func(c *gin.Context) {
		search := bson.M{}
		user := c.Query("user")
		seconds := c.Query("seconds")
		if user != "" {
			search["user"] = user
		}
		if seconds != "" {
			count, _ := strconv.Atoi(seconds)
			targetTime := time.Now().Add(time.Duration(-count) * time.Second)
			search["timestamp"] = bson.M{
				"$gt": targetTime,
			}
		}
		query := db.Find(search).Sort("$natural")
		var messages []Message
		query.All(&messages)
		if messages == nil {
			c.JSON(200, bson.M{})
		} else {
			c.JSON(200, messages)
		}
	})

	r.POST("api/post", func(c *gin.Context) {
		if c.Query("user") == "" || c.Query("text") == "" {
			log.Print("Invalid message.")
			return
		}
		var message Message
		message.User = c.Query("user")
		message.Text = c.Query("text")
		message.Timestamp = time.Now()
		if c.Query("ip") != "" {
			message.Ip = c.Query("ip")
		}
		db.Insert(message)
		msg, _ := json.Marshal(message)
		m.Broadcast(msg)
		c.JSON(200, bson.M{})
	})

	m.HandleMessage(func(s *melody.Session, msg []byte) {
		var message Message
		err := json.Unmarshal(msg, &message)
		if err != nil {
			log.Fatal(err)
		}
		ip, _, _ := net.SplitHostPort(s.Request.RemoteAddr)
		message.Ip = ip
		err = db.Insert(message)
		if err != nil {
			log.Println(err)
		}
		m.Broadcast(msg)
	})

	r.Run(":3000")
}
