// Command aircraft-replay serves a recorded adsb.lol answer for any area,
// so web and Apple work on the flights layer does not spend the feed's
// daily quota (ADR 28). Development only:
//
//	go run ./cmd/aircraft-replay            # listens on 127.0.0.1:18091
//	AIRCRAFT_FEED_URL=http://127.0.0.1:18091 shield serve
package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"shield/internal/sources/adsblol"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:18091", "listen address")
	flag.Parse()
	srv := &http.Server{Addr: *addr, Handler: adsblol.Replay(time.Now), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("aircraft replay on http://%s", *addr)
	log.Fatal(srv.ListenAndServe())
}
