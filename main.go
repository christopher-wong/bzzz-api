package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

const (
	gameCodeMin = 100000
	gameCodeMax = 999999
)

type player struct {
	GameID   int
	PlayerID int
	Name     string
}

type message struct {
	GameID   int    `json:"gameID"`
	PlayerID int    `json:"playerID"`
	Action   string `json:"action"`
}

var games map[int][](chan message)
var players map[int]player

var serverCh chan message

func init() {
	rand.Seed(time.Now().Unix())

	games = map[int][](chan message){}
	players = map[int]player{}
}

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/", IndexHandler).Methods("GET")
	r.HandleFunc("/host", HostCreateHandler).Methods("POST")
	r.HandleFunc("/host/{id}", HostListenHandler).Methods("GET")
	r.HandleFunc("/play/{id}", PlayHandler).Methods("GET")
	r.HandleFunc("/play/{id}/buzz", BuzzHandler).Methods("POST")

	corsH := handlers.CORS(handlers.AllowedOrigins([]string{"*"}))

	go func() {
		http.ListenAndServeTLS(":8080", "server.crt", "server.key", corsH(r))
	}()

	go func() {
		// select from the server channel forever
		// when a message comes in, grab it's game ID, and grab the client channels
		// for the given game id
		for {
			select {
			case msg, _ := <-serverCh:
				log.Printf("msg received: %v", msg)
				for _, clientCh := range games[msg.GameID] {
					clientCh <- msg
				}
			}
		}
	}()
}

// IndexHandler returns a static status 200 to verify server is running
func IndexHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Got connection: %s", r.Proto)
	w.WriteHeader(http.StatusOK)
}

// BuzzHandler returns a static status 200 to verify server is running
func BuzzHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Got connection: %s", r.Proto)
	log.Println("buzz detected")

	var clientMsg message
	json.NewDecoder(r.Body).Decode(&clientMsg)
	log.Printf("%v", clientMsg)

	serverCh <- clientMsg
	log.Printf("sent to client channel")
	w.WriteHeader(http.StatusCreated)
}

// HostCreateHandler handles a simple POST request to create a game instance
// and returns a game code.
func HostCreateHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Got connection: %s", r.Proto)

	gameCode := rand.Intn(gameCodeMax-gameCodeMin) + gameCodeMin
	if _, ok := games[gameCode]; ok {
		http.Error(w, "random game code collision. do a better job!", http.StatusInternalServerError)
		return
	}

	games[gameCode] = []chan message{}

	log.Printf("creating game: %d", gameCode)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]int{"gameCode": gameCode})
}

// PlayHandler establishes a stream and sends SSE to the client with
// game updates.
func PlayHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Got connection: %s", r.Proto)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// grab the game id from the path
	params := mux.Vars(r)
	id, ok := params["id"]
	if !ok {
		http.Error(w, "no 'id' found in URL", http.StatusBadRequest)
		return
	}

	// convert to an int
	i, err := strconv.Atoi(id)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, fmt.Sprintf("failed to convert game id [%s] to int", id), http.StatusInternalServerError)
		return
	}

	queryParams := r.URL.Query()
	playerName := queryParams.Get("name")

	// verify requested game exists
	_, ok = games[i]
	if !ok {
		log.Println("failed to verify that game exists")
		http.Error(w, fmt.Sprintf("game id [%s] not found", id), http.StatusBadRequest)
		return
	}
	log.Printf("listening to game: %d", i)

	// generate player id
	playerID := rand.Intn(gameCodeMax-gameCodeMin) + gameCodeMin
	if _, ok := players[playerID]; ok {
		log.Println(err.Error())
		http.Error(w, "random player id collision. do a better job!", http.StatusInternalServerError)
		return
	}

	players[playerID] = player{
		GameID:   i,
		PlayerID: playerID,
		Name:     playerName,
	}

	thisClientCh := make(chan message)
	games[i] = append(games[i], thisClientCh)

	// send initial messag
	resp := map[string]interface{}{
		"time":       time.Now().Local().String(),
		"gameID":     i,
		"playerID":   playerID,
		"playerName": playerName,
	}
	jsonBytes, err := json.Marshal(resp)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "data: %s\n\n", string(jsonBytes))
	flusher.Flush()
	// end initial message

	for {
		msg := <-thisClientCh
		resp := map[string]interface{}{
			"time":       time.Now().Local().String(),
			"gameID":     i,
			"playerID":   playerID,
			"playerName": playerName,
			"action":     msg.Action,
		}
		jsonBytes, err := json.Marshal(resp)
		if err != nil {
			log.Println(err.Error())
			http.Error(w, "failed to encode response", http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "data: %s\n\n", string(jsonBytes))
		flusher.Flush()
	}
}

// HostListenHandler establishes a stream and sends SSE related to host features.
func HostListenHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Got connection: %s", r.Proto)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	params := mux.Vars(r)
	id, ok := params["id"]
	if !ok {
		http.Error(w, "no 'id' found in URL", http.StatusBadRequest)
		return
	}

	i, err := strconv.Atoi(id)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, fmt.Sprintf("failed to convert game id [%s] to int", id), http.StatusInternalServerError)
		return
	}

	_, ok = games[i]
	if !ok {
		http.Error(w, fmt.Sprintf("game id [%s] not found", id), http.StatusBadRequest)
		return
	}

	log.Printf("HOST listening to game to game: %d", i)

	for {
		resp := map[string]interface{}{
			"gameID": i,
			"time":   time.Now().Local().String(),
		}
		jsonBytes, err := json.Marshal(resp)
		if err != nil {
			http.Error(w, "failed to encode response", http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "data: %s\n\n", string(jsonBytes))
		flusher.Flush()
		<-time.After(time.Second)
	}
}
