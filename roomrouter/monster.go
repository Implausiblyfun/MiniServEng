/*Package roomrouter copies https://github.com/200sc/selfPromotion/
*
 */
package roomrouter

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi"
)

var (
	gameLock sync.Mutex
	games    = make(map[string]Game)
)

// SetGameRoutes on the router
func SetGameRoutes() func(chi.Router) {
	return func(r chi.Router) {
		r.HandleFunc("/", gameBoilerPlate)
		r.Get("/list", gameLists)
		r.HandleFunc("/clear", gameClear)
		r.Group(func(r chi.Router) {
			r.Use(gameReqs)
			r.HandleFunc("/connect", gameConnect)
			r.HandleFunc("/disconnect", gameDisconnect)
			r.HandleFunc("/history", gameHistory)
			r.Get("/listen", gameListen)
			r.Post("/send", gameSend)
		})

	}
}

// Game is basically a chat room functionality.
type Game struct {
	players   map[string]struct{}
	listening map[string]chan []byte
	history   []historicalEvent
}

type historicalEvent struct {
	user    string
	payload string
}

type key int

const (
	gameIDKey key = iota
	pNameKey  key = iota
)

func gameReqs(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		gID := r.URL.Query().Get("gameID")
		if gID == "" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Println("couldn't find a gameID")
			fmt.Fprintf(w, "Bad Request Found, consult the parameter lists")
			return
		}
		pName := r.URL.Query().Get("name")
		if pName == "" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Println("couldn't find the users name")
			fmt.Fprintf(w, "Bad Request Found, consult the parameter lists")
			return
		}

		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = "000.000.0.0"
		}

		ctx := context.WithValue(
			context.WithValue(r.Context(), gameIDKey, gID),
			pNameKey, ip+pName)

		next.ServeHTTP(w, r.WithContext(ctx))
	})

}

// Begin Route creations

func gameLists(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Room list!\n"))
	operation := r.URL.Query().Get("players")
	if len(games) == 0 {
		fmt.Fprintf(w, "No rooms setup.")
		return
	}
	for game := range games {
		fmt.Fprintf(w, "- %s\n", game)
		if operation != "" {
			if len(games[game].players) == 0 {
				fmt.Fprintf(w, "No playtrs connected.")

			}
			for p := range games[game].players {
				fmt.Fprintf(w, "   %s\n", p)
			}

		}
	}
}

// TODO: make json to work
func gameHistory(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gID := ctx.Value(gameIDKey).(string)
	name := ctx.Value(pNameKey).(string)

	g, ok := games[gID]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	h := g.history
	title := "Game History for " + gID + " as requested by " + name

	format := req.URL.Query().Get("format")
	fmt.Println(title + " in format " + format)
	if strings.ToLower(format) == "json" {
		body, err := json.Marshal(h)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Something went real wrong in the history.")
			return
		}
		fmt.Fprintf(w, string(body))
		return
	}

	w.Write([]byte(title + ":\n"))
	w.Write([]byte("-----------------------\n"))
	for _, e := range h {
		pre := "-->"
		if e.user == name {
			pre = "<--"
		}
		fmt.Fprintf(w, "%s %s %s\n", e.user, pre, e.payload)
	}
	return
}

func gameBoilerPlate(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Currently used for mini20\n"))
	w.Write([]byte("Basically a chat room."))
}
func gameClear(w http.ResponseWriter, r *http.Request) {
	gameLock.Lock()
	defer gameLock.Unlock()
	gID := r.URL.Query().Get("gameID")
	_, ok := games[gID]
	if !ok {
		fmt.Printf("Failed to find the game='%s'\n", gID)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Failed to find the game:'%s' specified for Disconnection.", gID)
		return
	}

	delete(games, gID)
	fmt.Printf("Cleaning up the game %s\n", gID)
	fmt.Fprintf(w, "Cleaned up the game:%s!\n", gID)
	return
}

func gameConnect(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gID := ctx.Value(gameIDKey).(string)
	name := ctx.Value(pNameKey).(string)

	gameLock.Lock()
	defer gameLock.Unlock()
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Connecting to game")
	g, ok := games[gID]
	if ok {
		// connect to existing game
		fmt.Fprintf(w, "\nExisting game connected to.")
		g.players[name] = struct{}{}
	} else {
		// make new game
		g := Game{
			players:   map[string]struct{}{name: struct{}{}},
			listening: map[string]chan []byte{},
		}
		games[gID] = g
		fmt.Fprintf(w, "\nCreated a new game.")
	}

}

func gameListen(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gID := ctx.Value(gameIDKey).(string)
	name := ctx.Value(pNameKey).(string)

	gameLock.Lock()
	g, ok := games[gID]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		gameLock.Unlock()
		return
	}

	ch := make(chan []byte)
	g.listening[name] = ch
	gameLock.Unlock()
	select {
	case data := <-ch:
		_, err := w.Write(data)
		if err != nil {
			fmt.Println("failed to write data")
			w.WriteHeader(http.StatusInternalServerError)
		}
	case <-time.After(100 * time.Second):
		fmt.Println("in the future we would: checking livelyhood with a heartbeat request for ", gID, name)
		// TODO: consider setting something here to actually make this
		w.WriteHeader(http.StatusRequestTimeout)
	}

	// This should indicate the user stopped listening.
	// They may reconnect eventually but maybe not so we will have to make this clearer in the future.
	// Later sweep should deal with sending to others in the game to decide if they leave or what.
	gameLock.Lock()
	delete(g.listening, name)
	gameLock.Unlock()
}

func gameSend(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gID := ctx.Value(gameIDKey).(string)
	thisPlayer := ctx.Value(pNameKey).(string)

	gameLock.Lock()
	defer gameLock.Unlock()
	g, ok := games[gID]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		fmt.Println("failed to read data")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	g.history = append(g.history, historicalEvent{thisPlayer, string(body)})
	games[gID] = g
	for name, ch := range g.listening {
		if name == thisPlayer {
			continue
		}
		select {
		case ch <- body:
		case <-time.After(3 * time.Second):
			fmt.Println("failed to send data")
		}
	}
}

func gameDisconnect(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gID := ctx.Value(gameIDKey).(string)
	name := ctx.Value(pNameKey).(string)

	gameLock.Lock()
	defer gameLock.Unlock()
	g, ok := games[gID]
	if !ok {
		fmt.Printf("Failed to find the game=%s, pname=%s\n", gID, name)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Failed to find the game specified for Disconnection.")
		return
	}
	if _, ok := g.players[name]; !ok {
		fmt.Printf("Failed to find the player gid=%s, pname=%s\n", gID, name)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Failed to find the player specified for Disconnection.")
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Disconnected from the game.\n")

	delete(g.players, name)
	if len(g.players) == 0 {
		delete(games, gID)
		fmt.Printf("Cleaning up the game %s\n", gID)
		fmt.Fprintf(w, "Cleaned up the game:%s!\n", gID)
	}
}
