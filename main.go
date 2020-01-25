package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/implausiblyfun/miniserveng/roomrouter"
)

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Throttle(3))
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Plz dont overload this.\nJust trying to make a nice little easy webbysite."))
	})

	r.Route("/game", roomrouter.SetGameRoutes())

	port := "970"
	fmt.Printf("Running on port %s\n", port)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}

}
