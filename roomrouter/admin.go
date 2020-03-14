package roomrouter

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi"
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

// list the game rooms set up in this server.
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

// clear up a game and wipe it from existance.
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

//
func gameBoilerPlate(w http.ResponseWriter, req *http.Request) {
	w.Write([]byte("Currently used for mini20:\n"))
	gameLists(w, req)
}
