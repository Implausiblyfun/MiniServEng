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

type key int

const (
	gameIDKey key = iota
	pNameKey  key = iota
)

// SetGameRoutes onto a provided router.
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
	players   map[string]*player
	listening map[string]chan []byte
	history   []historicalEvent
	gameEnd   chan bool
}

type historicalEvent struct {
	user    string
	payload string
}

// player is a simple container for information.
// Will overhaul things like playorder later.
type player struct {
	name      string
	playOrder int
	lastSeen  time.Time
}

func (p *player) seen() {
	p.lastSeen = time.Now()
}

func toPlayerIdentifier(ip, name string) string {
	return ip + "|" + name
}
func toPlayerName(playerID string) string {
	pComponents := strings.Split(playerID, "|")
	if len(pComponents) < 2 {
		return ""
	}
	return strings.Join(pComponents[1:], "|")
}

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
			pNameKey, toPlayerIdentifier(ip, pName))

		next.ServeHTTP(w, r.WithContext(ctx))
	})

}

// Begin Route creations

func gameLists(w http.ResponseWriter, req *http.Request) {
	w.Write([]byte("Room list!\n"))
	operation := req.URL.Query().Get("players")
	if len(games) == 0 {
		fmt.Fprintf(w, "No rooms setup.")
		return
	}
	for game := range games {
		fmt.Fprintf(w, "- %s\n", game)
		if operation != "" {
			if len(games[game].players) == 0 {
				fmt.Fprintf(w, "   No players connected.")

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

func gameBoilerPlate(w http.ResponseWriter, req *http.Request) {
	w.Write([]byte("Currently used for mini20\n"))
	w.Write([]byte("Basically a chat room."))
}
func gameClear(w http.ResponseWriter, req *http.Request) {
	gameLock.Lock()
	defer gameLock.Unlock()
	gID := req.URL.Query().Get("gameID")
	g, ok := games[gID]
	if !ok {
		fmt.Printf("Failed to find the game='%s'\n", gID)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Failed to find the game:'%s' specified for Disconnection.", gID)
		return
	}
	g.gameEnd <- true
	fmt.Fprintf(w, "Cleaning up the game:%s!\n", gID)
	return
}

const checkSeconds = 40

func gameConnect(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gID := ctx.Value(gameIDKey).(string)
	name := ctx.Value(pNameKey).(string)

	gameLock.Lock()
	defer gameLock.Unlock()
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Connecting to game %s\n", gID)
	g, ok := games[gID]
	if ok {
		// connect to existing game
		fmt.Fprintf(w, "Existing game %s connected to.\n", gID)
		g.players[name] = &player{name, -1, time.Now()}

	} else {
		// make new game
		g = Game{
			players:   map[string]*player{name: &player{name, -1, time.Now()}},
			listening: map[string]chan []byte{},
			gameEnd:   make(chan bool, 2),
		}
		games[gID] = g

		// Start the monitoring functionality to check with heartybeatz.
		go func() {
			for {
				fmt.Printf("check game %s\n", gID)
				select {
				case <-g.gameEnd:
					fmt.Println("Game End detected. Cleaning up as much as we can.")
					gameLock.Lock()
					delete(games, gID)
					gameLock.Unlock()
					fmt.Printf("Cleaning up the game %s\n", gID)
					return
				case <-time.After(checkSeconds * time.Second):

					heartbeatDelay := checkSeconds * 2 * time.Second
					heartbeatCutOff := time.Now().Add(heartbeatDelay * -1)
					for _, p := range g.players {
						if p.lastSeen.After(heartbeatCutOff) {
							continue
						}

						fmt.Printf("Asking if %s is still there in %s :( \n", p.name, gID)
						ayste := Event{Name: "HeartbeatCheck", Payload: AreYouStillThereEvent{ConnectionParameters{"THE SERVER"}}}
						notAPassiveAgressiveHeartBeat, _ := json.Marshal(ayste)
						select {
						case g.listening[p.name] <- notAPassiveAgressiveHeartBeat:
						case <-time.After(4 * time.Second):
							fmt.Println("failed to send data to ", name)
						}
						if p.lastSeen.After(heartbeatCutOff.Add(heartbeatDelay * -1)) {
							continue
						}
						_, err := w.Write(notAPassiveAgressiveHeartBeat)
						if err != nil {
							fmt.Println("failed to write data for heartbeat")
						}

						fmt.Printf("Removing player %s from game %s \n", p.name, gID)
						gameLock.Lock()
						delete(g.players, p.name)
						delete(g.listening, p.name)
						gameLock.Unlock()
						if len(g.players) == 0 {
							g.gameEnd <- true
						}
					}
				}
			}
		}()
		fmt.Fprintf(w, "Created a new game.\n")
	}

	// make the listening chan here
	//TODO:take out from listening?
	var ch chan []byte
	if ch, ok = g.listening[name]; !ok || ch == nil {
		ch = make(chan []byte, 20)
		g.listening[name] = ch
	}

	pce := Event{Name: "PlayerConnected", Payload: PlayerConnectedEvent{ConnectionParameters{"THE SERVER"}, toPlayerName(name)}}
	pc, err := json.Marshal(pce)
	if err != nil {
		fmt.Println(err)
		return
	}
	g.sendToParticipants(name, pc)

	for otherN := range g.listening {
		fmt.Println("Trying to send to er the person called", otherN)
		if name == otherN {
			continue
		}
		pce.Payload = PlayerConnectedEvent{ConnectionParameters{toPlayerName(otherN)}, toPlayerName(otherN)}
		body, err := json.Marshal(pce)
		if err != nil {
			fmt.Println(err)
			continue
		}
		fmt.Println("No problemos sending to for ", name, otherN)
		select {
		case ch <- body:
		case <-time.After(4 * time.Second):
			fmt.Println("failed to send data for ", otherN)
		}
	}

	fmt.Printf("Game %s now has the following players %v.\n", gID, g.players)
	// decide who goes first and send the appropriate info.
	if len(g.players) == 2 {

		starts := true
		for name, ch := range g.listening {
			poe := Event{Name: "PlayOrder", Payload: PlayOrderEvent{ConnectionParameters{"THE SERVER"}, starts}}

			body, err := json.Marshal(poe)
			if err != nil {
				fmt.Println(err)
				continue
			}

			select {
			case ch <- body:
			case <-time.After(4 * time.Second):
				fmt.Printf("failed to send data for starting to %s.\n", name)
			}
			starts = false
		}
	}
}

func gameListen(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gID := ctx.Value(gameIDKey).(string)
	name := ctx.Value(pNameKey).(string)

	// playStatus := req.URL.Query().Get("pStatus")

	fmt.Printf("Listening for %s:%s\n", gID, name)
	gameLock.Lock()
	g, ok := games[gID]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		gameLock.Unlock()
		return
	}

	var ch chan []byte
	if ch, ok = g.listening[name]; !ok || ch == nil {
		ch = make(chan []byte, 20)
		g.listening[name] = ch
	}
	gameLock.Unlock()

	select {
	case data := <-ch:
		// fmt.Println("Got DATA for ", name, string(data))
		gameLock.Lock()
		g.players[name].seen()
		gameLock.Unlock()
		_, err := w.Write(data)
		if err != nil {
			fmt.Println("failed to write data down to ", g.players[name])
			w.WriteHeader(http.StatusInternalServerError)
		}
	case <-time.After(checkSeconds * 4 * time.Second):
		fmt.Printf("No data sent in the last %d seconds for %s:%s\n", 100, gID, name)
		w.WriteHeader(http.StatusGatewayTimeout)
	}

	gameLock.Lock()
	// delete(g.listening, name)
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
		fmt.Println("failed to read data sent from ", thisPlayer)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	fmt.Printf("recieved a send from %s: %s", thisPlayer, string(body))
	if _, ok := games[gID].players[thisPlayer]; ok {
		games[gID].players[thisPlayer].lastSeen = time.Now()
	}

	var ev Event
	err = json.Unmarshal(body, &ev)
	if err == nil {
		switch ev.Name {
		case "HeartbeatCheck":
			return //shortcircuit for heartybeat checks
		}

	}

	g.history = append(g.history, historicalEvent{thisPlayer, string(body)})
	games[gID] = g
	g.sendToParticipants(thisPlayer, body)

}

func (g *Game) sendToParticipants(playerName string, body []byte) {
	for name, ch := range g.listening {
		if name == playerName {
			continue
		}
		select {
		case ch <- body:
		case <-time.After(4 * time.Second):
			fmt.Printf("failed to send data to %s.\n", name)
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
	delete(g.listening, name)
	if len(g.players) == 0 {
		g.gameEnd <- true
	}
}
