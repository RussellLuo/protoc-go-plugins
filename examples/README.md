# Hello World Example

## Download the example

```bash
$ go get google.golang.org/grpc
```

See [here][1] for more details.

## Generate the HTTP service code

```bash
$ cd $GOPATH/src/google.golang.org/grpc/examples/helloworld
$ protoc helloworld/helloworld.proto --gohttp_out=pb_pkg_path=google.golang.org/grpc/examples/helloworld/helloworld:greeter_server
$ mv greeter_server/helloworld greeter_server/http
```

## Update the original gRPC server code

```bash
$ cd $GOPATH/src/google.golang.org/grpc/examples/helloworld
$ vi greeter_server/main.go
```

1. Add the following import:

    ```go
    import (
        ...
        "google.golang.org/grpc/examples/helloworld/greeter_server/http"
        ...
    )
    ```

2. Add the following constant variable:

    ```go
    const (
            ...
            httpPort = ":50052"
    )
    ```

3. Add the following code at the start of `main()`:

    ```go
    func main() {
            h := http.NewServer(&server{})
            go func() {
                    if err := h.Serve(httpPort); err != nil {
                            log.Fatalf("failed to start the HTTP server: %v", err)
                    }
            }()

            ...
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
$ curl -i -H 'Content-Type: application/json' -XPOST http://localhost:50052/greeter/say_hello -d '{"name": "world"}'
```

Or by [HTTPie][2]:

```bash
$ http post http://localhost:50052/greeter/say_hello name=world
```


[1]: http://www.grpc.io/docs/quickstart/go.html#download-the-example
[2]: https://github.com/jakubroztocil/httpie
