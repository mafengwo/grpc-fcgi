package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/pkg/errors"
	"github.com/mafengwo/grpc-fcgi/test/client/route_guide"
	"log"
	"math/rand"
	"os"
	"strconv"
	"time"

	"google.golang.org/grpc/metadata"

	"google.golang.org/grpc"
)

var (
	addr        = flag.String("addr", "localhost:8080", "grpc server address")
	times       = flag.Int("count", 1, "loop times")
	concurrency = flag.Int("concurrency", 1, "concurrency")
	method      = flag.String("method", "", "method for test")
	timeout     = flag.Int("timeout", 10, "timeout for call")

	client route_guide.RouteGuideClient
)

func main() {
	flag.Parse()

	conn, err := grpc.Dial(*addr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	client = route_guide.NewRouteGuideClient(conn)

	start := time.Now()

	goon := true
	cc := make(chan int, *concurrency)
	for i := 0; i < *times && goon; i++ {
		cc <- 1

		go func(i int) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(*timeout))
			defer cancel()

			rid := strconv.Itoa(i) + "#" + strconv.Itoa(time.Now().Nanosecond()) + strconv.Itoa(rand.Int())
			header := metadata.New(map[string]string{
				"client":     "go-proxy-test",
				"request_id": rid,
			})
			ctx = metadata.NewOutgoingContext(ctx, header)

			var respHeader, respTrailer metadata.MD
			var err error
			switch *method {
			case "get_feature":
				err = getFeature(ctx, &respHeader, &respTrailer)
				if err != nil {
					fmt.Println(err.Error())
				}
				fmt.Printf("header: %+v, trailer: %+v\n", respHeader, respTrailer)
				os.Exit(1)
				break
			case "list_features":
				err = listFeatures(ctx, &respHeader, &respTrailer)
				break
			case "record_route":
				err = recordRoute(ctx, &respHeader, &respTrailer)
				break
			case "route_chat":
				break
			default:
				fmt.Println("method unsupported")
				return
			}

			goon = err == nil

			if err != nil {
				fmt.Println(err.Error())
				fmt.Printf("header: %+v, trailer: %+v\n", respHeader, respTrailer)
			}

			<-cc
		}(i)
	}

	for j := 0; j < *concurrency; j++ {
		cc <- 1
	}
	cost := time.Now().Sub(start)
	log.Printf("cost: %s; qps: %.0f", cost, float64(*times) / cost.Seconds())
}

func getFeature(ctx context.Context, header *metadata.MD, trailer *metadata.MD) error {
	p := &route_guide.Point{
		Latitude: rand.Int31(),
		Longitude: rand.Int31(),
	}

	resp, err := client.GetFeature(ctx, p, grpc.Header(header), grpc.Trailer(trailer))
	if err != nil {
		return err
	}

	if resp.Location.Longitude != p.Longitude || resp.Location.Latitude != p.Latitude {
		return errors.Errorf(
			"output mismatch input. input: (lat: %d; lng: %d), output: (lat: %d; lng: %d)",
			p.Latitude, p.Longitude, resp.Location.Latitude, resp.Location.Longitude)
	}

	return nil
}

func listFeatures(ctx context.Context, header *metadata.MD, trailer *metadata.MD) error {
	p := &route_guide.Rectangle{
		Lo: &route_guide.Point{
			Latitude: rand.Int31(),
			Longitude: rand.Int31(),
		},
		Hi:&route_guide.Point{
			Latitude: rand.Int31(),
			Longitude: rand.Int31(),
		},
	}

	fc, err := client.ListFeatures(ctx, p, grpc.Header(header), grpc.Trailer(trailer))
	if err != nil {
		return err
	}

	for {
		resp, err := fc.Recv()

		if err != nil {
			return err
		}

		if resp == nil {
			return errors.Errorf("resp is nil")
		}
	}

	return nil
}

func recordRoute(ctx context.Context, header *metadata.MD, trailer *metadata.MD) error {
	rrc, err := client.RecordRoute(ctx)
	if err != nil {
		return err
	}

	p := &route_guide.Point{
		Latitude: rand.Int31(),
		Longitude: rand.Int31(),
	}

	times := 10
	for i := 0; i < times; i++ {
		err = rrc.Send(p)
		if err != nil {
			return err
		}
	}

	resp, err := rrc.CloseAndRecv()
	if err != nil {
		return  err
	}
	if int(resp.PointCount) != times {
		return errors.Errorf("sent %d, received: %d", times, resp.PointCount)
	}
	return nil
}
