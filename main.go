package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"time"

	_ "image/png"

	"github.com/faiface/pixel"
	"github.com/faiface/pixel/pixelgl"
	"github.com/gorilla/websocket"
	"golang.org/x/image/colornames"
)

func loadPicture(path string) (pixel.Picture, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}
	return pixel.PictureDataFromImage(img), nil
}

type Player struct {
	sprite     *pixel.Sprite
	DrawMatrix pixel.Matrix
	PlayerId   int
}

type World struct {
	Players [4]*Player
}

type LocalContext struct {
	currentPlayer int
}

var playerId int

func getCurrentPlayerId() int {
	return playerId
}

func updatePlayerPosition(playerId int, playerPosition pixel.Matrix) {
	world.Players[playerId].DrawMatrix = playerPosition
}

var world World

func run() {
	cfg := pixelgl.WindowConfig{
		Title:  "Pixel Rocks! Player " + fmt.Sprint(getCurrentPlayerId()),
		Bounds: pixel.R(0, 0, 1024, 768),
		VSync:  true,
	}

	win, err := pixelgl.NewWindow(cfg)
	if err != nil {
		panic(err)
	}

	pic, err := loadPicture("resources/players.png")
	if err != nil {
		panic(err)
	}

	players := [4]*Player{}
	players[0] = &Player{pixel.NewSprite(pic, pixel.R(1, 0, 49, 50)), pixel.IM.Moved(pixel.Vec{X: 250, Y: 250}), 0}
	players[1] = &Player{pixel.NewSprite(pic, pixel.R(50, 0, 98, 50)), pixel.IM.Moved(pixel.Vec{X: 750, Y: 250}), 1}
	players[2] = &Player{pixel.NewSprite(pic, pixel.R(99, 0, 147, 50)), pixel.IM.Moved(pixel.Vec{X: 250, Y: 500}), 2}
	players[3] = &Player{pixel.NewSprite(pic, pixel.R(148, 0, 196, 50)), pixel.IM.Moved(pixel.Vec{X: 750, Y: 500}), 3}

	localContext := &LocalContext{getCurrentPlayerId()}
	world.Players = players

	if getCurrentPlayerId() == 0 {
		go hostServer()
	} else {
		go runClient()
	}

	for !win.Closed() {
		win.Update()

		win.Clear(colornames.Skyblue)

		updatePlayerPosition(localContext.currentPlayer, pixel.IM.Moved(win.MousePosition()))

		for player := range world.Players {
			players[player].sprite.Draw(win, players[player].DrawMatrix)
		}
	}
}

var addr = flag.String("addr", "localhost:8080", "http service address")
var upgrader = websocket.Upgrader{}

func hostServer() {
	flag.Parse()
	log.SetFlags(0)
	http.HandleFunc("/player", serverPlayerPosition)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
func runClient() {
	flag.Parse()
	log.SetFlags(0)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	u := url.URL{Scheme: "ws", Host: *addr, Path: "/player"}
	log.Printf("connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	ticker := time.NewTicker(10)

	for t := range ticker.C {
		msg, err := json.Marshal(world.Players[getCurrentPlayerId()])
		if err != nil {
			log.Println("write:", err)
			log.Println(t)
			break
		}

		err = c.WriteMessage(2, msg)
		if err != nil {
			log.Println("write:", err)
			break
		}

		mt, message, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			log.Println(mt)
			break
		}

		serverWorld := &World{}
		json.Unmarshal(message, serverWorld)

		for i, player := range serverWorld.Players {
			updatePlayerPosition(i, player.DrawMatrix)
		}
	}
}

func serverPlayerPosition(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()
	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}

		player := &Player{}
		json.Unmarshal(message, player)

		updatePlayerPosition(player.PlayerId, player.DrawMatrix)

		msg, err := json.Marshal(world)
		if err != nil {
			log.Println("write:", err)
			break
		}

		err = c.WriteMessage(mt, msg)
		if err != nil {
			log.Println("write:", err)
			break
		}
	}
}

func main() {
	fmt.Fscan(os.Stdin, &playerId)

	pixelgl.Run(run)
}
