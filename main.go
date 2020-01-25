package main

import (
	"fmt"
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

	fmt.Println("Running on port  289")
	http.ListenAndServe(":289", r)

}
