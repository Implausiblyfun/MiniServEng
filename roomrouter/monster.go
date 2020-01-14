/*Package roomrouter copies https://github.com/200sc/selfPromotion/
*
 */
package roomrouter

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
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
		r.Get("/connect", gameConnect)
		r.Get("/disconnect", gameDisconnect)
		r.Get("/listen", gameListen)
		r.Get("/send", gameSend)
	}
}

// Game is basically a chat room functionality.
type Game struct {
	players   map[string]struct{}
	listening map[string]chan []byte
}

func parseGameID(req *http.Request) (string, error) {
	val := req.URL.Query().Get("gameID")
	if val == "" {
		return "", errors.New("no game id field found")
	}
	return val, nil
}

func getPlayerName(req *http.Request) string {
	return req.RemoteAddr + req.URL.Query().Get("name")
}

func gameConnect(w http.ResponseWriter, req *http.Request) {
	gID, err := parseGameID(req)
	if err != nil {
		fmt.Println("couldn't parse game id", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	name := getPlayerName(req)

	gameLock.Lock()
	defer gameLock.Unlock()
	g, ok := games[gID]
	if ok {
		// connect to existing game
		g.players[name] = struct{}{}
	} else {
		// make new game
		g := Game{
			players:   map[string]struct{}{name: struct{}{}},
			listening: map[string]chan []byte{},
		}
		games[gID] = g
	}
	w.WriteHeader(http.StatusOK)
}

func gameListen(w http.ResponseWriter, req *http.Request) {
	gID, err := parseGameID(req)
	if err != nil {
		fmt.Println("couldn't parse game id", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	gameLock.Lock()
	g, ok := games[gID]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		gameLock.Unlock()
		return
	}
	name := getPlayerName(req)
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
		fmt.Println("timed out")
		w.WriteHeader(http.StatusRequestTimeout)
	}
	gameLock.Lock()
	delete(g.listening, name)
	gameLock.Unlock()
}

func gameSend(w http.ResponseWriter, req *http.Request) {
	gID, err := parseGameID(req)
	if err != nil {
		fmt.Println("couldn't parse game id", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
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
	thisPlayer := getPlayerName(req)
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
	gID, err := parseGameID(req)
	if err != nil {
		fmt.Println("couldn't parse game id", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	gameLock.Lock()
	defer gameLock.Unlock()
	g, ok := games[gID]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	name := getPlayerName(req)

	delete(g.players, name)
	if len(g.players) == 0 {
		delete(games, gID)
	}
	w.WriteHeader(http.StatusOK)
}
