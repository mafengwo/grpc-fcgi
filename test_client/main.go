package main

import (
	"context"
	"flag"
	"log"
	"math/rand"
	"strconv"
	"time"

	"gitlab.mfwdev.com/service/grpc-fcgi/test_client/flight_price"
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

	start := time.Now()

	goon := true
	cc := make(chan int, *concurrency)
	for i := 0; i < *times && goon; i++ {
		cc <- 1

		go func(i int) {
			arriveId, departureId := rand.Uint64(), rand.Uint64()
			req := &flight_price.CityCheapestPriceRequest{
				DepartureCityID: departureId,
				ArriveCityID:    arriveId,
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
			defer cancel()
			header := metadata.New(map[string]string{
				"client": "go-proxy-test",
				"index": strconv.Itoa(i),
			})
			ctx = metadata.NewOutgoingContext(ctx, header)

			var respHeader, respTrailer metadata.MD
			cli := flight_price.NewPriceClient(conn)
			reply, err := cli.GetCityCheapestPrice(ctx, req, grpc.Header(&respHeader), grpc.Trailer(&respTrailer))
			if err != nil {
				log.Printf("%d failed: %v", i, err)
				goon = false
			} else if reply.GetArriveCityID() != arriveId || reply.DepartureCityID != departureId {
				log.Printf("%d input: %d %d, output: %d %d reply: %v",
					i, arriveId, departureId, reply.GetArriveCityID(), reply.GetDepartureCityID(), reply)
				goon = false
			} else {
				/*
				log.Printf("arrive id: %d\ndeparture id: %d\nprice: %f\nconcurrent:%s\n",
					reply.GetArriveCityID(),
					reply.GetDepartureCityID(),
					reply.GetPrice(),
					reply.GetConcurrency())
				log.Printf("header: %v\n trailer:%v", respHeader, respTrailer)
				*/
			}

			<-cc
		}(i)
	}

	for j := 0; j < *concurrency; j++ {
		cc <- 1
	}
	cost := time.Now().Sub(start)
	log.Printf("cost: %s", cost)
}
