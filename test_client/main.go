package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/bakins/grpc-fastcgi-proxy/test_client/flight_price"
	"google.golang.org/grpc/metadata"

	"google.golang.org/grpc"
)

var (
	addr        = flag.String("addr", "localhost:8080", "grpc server address")
	times       = flag.Int("count", 1, "loop times")
	concurrency = flag.Int("concurrency", 1, "concurrency")
)

func main() {
	flag.Parse()

	conn, err := grpc.Dial(*addr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	cc := make(chan int, *concurrency)
	for i := 0; i < *times; i++ {
		cc <- 1

		go func() {
			cli := flight_price.NewPriceClient(conn)
			req := &flight_price.CityCheapestPriceRequest{
				DepartureCityID: 1,
				ArriveCityID:    2,
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*1)
			defer cancel()
			header := metadata.New(map[string]string{
				"client": "go-proxy-test",
			})
			ctx = metadata.NewOutgoingContext(ctx, header)

			var respHeader, respTrailer metadata.MD
			reply, err := cli.GetCityCheapestPrice(ctx, req, grpc.Header(&respHeader), grpc.Trailer(&respTrailer))
			if err != nil {
				log.Printf("failed: %v", err)
			}
			log.Printf("reply: %+v; header: %+v; trailer: %+v", reply, respHeader, respTrailer)

			<-cc
		}()
	}

	for j := 0; j < *concurrency; j++ {
		cc <- 1
	}
}
