package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffegrpc/grpccredentials"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/examples/helloworld/helloworld"
	"google.golang.org/grpc/peer"
)

func main() {
	var addr string
	flag.StringVar(&addr, "addr", "", "host:port of the server")
	flag.Parse()

	if addr == "" {
		addr = os.Getenv("GREETER_SERVER_ADDR")
	}
	if addr == "" {
		addr = "localhost:8080"
	}

	log.Println("Starting up...")
	log.Println("Server Address:", addr)

	ctx := context.Background()

	source, err := workloadapi.NewX509Source(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer source.Close()

	serverID := spiffeid.RequireFromString("spiffe://cluster1.demo/greeter-server")

	creds := grpccredentials.MTLSClientCredentials(source, source, tlsconfig.AuthorizeID(serverID))

	client, err := grpc.DialContext(ctx, addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	greeterClient := helloworld.NewGreeterClient(client)

	const interval = time.Second * 10
	log.Printf("Issuing requests every %s...", interval)
	for {
		issueRequest(ctx, greeterClient)
		time.Sleep(interval)
	}
}

func issueRequest(ctx context.Context, c helloworld.GreeterClient) {
	p := new(peer.Peer)
	resp, err := c.SayHello(ctx, &helloworld.HelloRequest{
		Name: "Joe",
	}, grpc.Peer(p))
	if err != nil {
		log.Printf("Failed to say hello: %v", err)
		return
	}

	/////////////////////////////////////////////////////////////////////////
	// TODO: Obtain the server SPIFFE ID
	/////////////////////////////////////////////////////////////////////////
	serverID := "SOME-SERVER"
	if peerID, ok := grpccredentials.PeerIDFromPeer(p); ok {
		serverID = peerID.String()
	}

	log.Printf("%s said %q", serverID, resp.Message)
}
