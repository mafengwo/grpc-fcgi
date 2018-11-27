package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/kellegous/fcgi"
)

func main() {
	wd, err := os.Getwd()
	if err != nil {
		log.Panic(err)
	}

	c, err := fcgi.NewClient("unix", "/run/php/php7.0-fpm.sock")
	if err != nil {
		log.Panic(err)
	}

	http.HandleFunc("/",
		func(w http.ResponseWriter, r *http.Request) {
			params := fcgi.ParamsFromRequest(r)
			params["SCRIPT_FILENAME"] = []string{filepath.Join(wd, "index.php")}
			c.ServeHTTP(params, w, r)
		})
	log.Panic(http.ListenAndServe(":80", nil))
}
