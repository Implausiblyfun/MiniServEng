/*Package roomrouter copies https://github.com/200sc/selfPromotion/
*
 */
package roomrouter

import (
	"context"
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
		r.HandleFunc("/", gameBoilerPlate)
		r.Get("/list", gameLists)
		r.Group(func(r chi.Router) {
			r.Use(gameReqs)
			r.Post("/connect", gameConnect)
			r.HandleFunc("/disconnect", gameDisconnect)
			r.Get("/listen", gameListen)
			r.Post("/send", gameSend)
		})

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

		ctx := context.WithValue(
			context.WithValue(r.Context(), gameIDKey, gID),
			pNameKey, r.RemoteAddr+pName)

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

func gameBoilerPlate(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Currently used for mini20\n"))
	w.Write([]byte("Basically a chat room."))
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
		fmt.Println("timed out")
		w.WriteHeader(http.StatusRequestTimeout)
	}
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
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	delete(g.players, name)
	if len(g.players) == 0 {
		delete(games, gID)
	}
	w.WriteHeader(http.StatusOK)
}
