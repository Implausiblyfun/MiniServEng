package main

import (
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/implausiblyfun/miniserveng/roomrouter"
)

func main() {
	r := chi.NewRouter()
	r.Use(middleware.ThrottleBacklog(10, 50, time.Second*5))

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Plz dont overload this.\nJust trying to make a nice little easy webbysite."))
	})

	r.Route("/game", roomrouter.SetGameRoutes())
	r.HandleFunc("/ref", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Try to ref?"))
		cmd := exec.Command("bash", "~updater.sh")
		err := cmd.Run()
		fmt.Fprintf(w, err.Error())
	})
	port := "8080"
	fmt.Printf("Running on port %s\n", port)

	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(err)
	}

}
