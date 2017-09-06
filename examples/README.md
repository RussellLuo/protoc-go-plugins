# Hello World Example

## Prerequisites

### Install cmux

```bash
$ go get -u github.com/soheilhy/cmux
```

### Download the example

```bash
$ go get google.golang.org/grpc
```

See [here][1] for more details.

## Generate the HTTP server code

```bash
$ cd $GOPATH/src/google.golang.org/grpc/examples/helloworld
$ protoc helloworld/helloworld.proto --gohttp_out=pb_pkg_path=google.golang.org/grpc/examples/helloworld/helloworld:greeter_server
$ mv greeter_server/helloworld greeter_server/http
```

For those who want to preview the final generated code, see the pre-generated file [helloworld.http.go](helloworld.http.go).

## Update the existing gRPC server code

```bash
$ cd $GOPATH/src/google.golang.org/grpc/examples/helloworld
$ vi greeter_server/main.go
```

1. Add the following imports:

    ```go
    import (
    	...
    	"github.com/soheilhy/cmux"
    	"google.golang.org/grpc/examples/helloworld/greeter_server/http"
    	...
    )
    ```

2. Change the function `main()` as follows:

    ```go
    func main() {
    	lis, err := net.Listen("tcp", port)
    	if err != nil {
    		log.Fatalf("failed to listen: %v", err)
    	}

    	m := cmux.New(lis)
    	// Using MatchWithWriters/SendSettings is a major performance hit (around 15%).
    	// Per the cmux documentation, you have to do this for grpc-java.
    	// If only using golang, you don't need this, but probably not
    	// great to assume what the calling languages are.
    	grpcL := m.MatchWithWriters(cmux.HTTP2MatchHeaderFieldPrefixSendSettings("content-type", "application/grpc"))
    	httpL := m.Match(cmux.Any())

    	srv := &server{}

    	// You can also set an unary server interceptor here
    	httpS := http.NewServer()
    	httpS.RegisterGreeterServer(srv)
    	go func() {
    		if err := httpS.Serve(httpL); err != nil {
    			log.Fatalf("failed to start HTTP server listening: %v", err)
    		}
    	}()

    	grpcS := grpc.NewServer()
    	pb.RegisterGreeterServer(grpcS, srv)
    	// Register reflection service on gRPC server.
    	reflection.Register(grpcS)
    	go func() {
    		if err := grpcS.Serve(grpcL); err != nil {
    			log.Fatalf("failed to serve: %v", err)
    		}
    	}()

    	m.Serve()
    }
    ```

## Start the HTTP (and gRPC) server

```bash
$ cd $GOPATH/src/google.golang.org/grpc/examples/helloworld
$ go run greeter_server/main.go
```

## Consume the HTTP service

By cURL:

```bash
$ curl -i -H 'Content-Type: application/json' -XPOST http://localhost:50051/greeter/say_hello -d '{"name": "world"}'
```

Or by [HTTPie][2]:

```bash
$ http post http://localhost:50051/greeter/say_hello name=world
```


[1]: http://www.grpc.io/docs/quickstart/go.html#download-the-example
[2]: https://github.com/jakubroztocil/httpie
