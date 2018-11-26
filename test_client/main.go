package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/bakins/grpc-fastcgi-proxy/test_client/flight_price"
	"log"
	"time"

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
	fmt.Printf("connected\n")
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
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*12)
			defer cancel()

			reply, err := cli.GetCityCheapestPrice(ctx, req)
			if err != nil {
				log.Printf("failed: %v", err)
			} else {
				log.Printf("reply: %+v", reply)
			}

			<-cc
		}()
	}

	for j := 0; j < *concurrency; j++ {
		cc <- 1
	}
}
