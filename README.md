# Baked Baker

## Introduction

Baked Baker (BB) is a HTTP proxy that directs requests to Agent Baker instances at different versions.

There is a need to have Agent Baker instances that are stable for some length of time for a few select customers with tight requirements. This allows those customers to use the same Agent Baker instance for some predetermined length of time while others use the latest version.

The version here isn't production quality, it is a proof of concept to show that the idea is possible. There is no support for this code and it is not recommended for production use.

## Internals

### Embeded versions

BB works by having embedded Agent Baker instances at different versions. This comes from the `internal/versions/binaries` directory and is mounted as an `embed.FS` filesystem.

The directories in `internal/versions/binaries` are named after the version of the Agent Baker instance. If the directory is not named in the Agent Baker version format, the binary will panic on start.

Inside the directory, there should be a single binary named `agentbaker`. Each binary is started with the `--port` flag to specify the port it should listen on. Each instance will listen on a different `localhost` port.

If any instances fail to start, the binary will panic and exit.

### RPC routing

BB supports the same 3 REST RPC calls that Agent Baker does. These are:

- /getnodebootstrapdata
- /getlatestsigimageconfig
- /getdistrosigimageconfig

If the RPC call contains the standard RPC data for a standard Agent Baker call, the call is routed to the latest version of Agent Baker.

If the RPC calls uses the JSON format of:

```go
type VersionedReq[T any] struct {
	// ABVersion is the Agent Baker version. This must be set to a valid version
	// or "latest".
	ABVersion versions.Version
	// Req is the request to be sent to the agent baker service.
	Req T
}
```

The call is routed to the specified version of Agent Baker.

![Flow Diagram](https://github.com/element-of-surprise/bakedbaker/blob/main/docs/bakedbaker-flow.pngg)

BB's flow is a simplistic proxy with nothing special over a regular proxy other that it routes requests to different versions of Agent Baker based on the request.

### Implementation Details

A few things will stand out in this implementation:

- The use of `gofiber` for the HTTP server.
- Variable argument types for the RPC calls instead of separate RPC calls.
- The use of the `embed.FS` filesystem for the embedded Agent Baker instances.
- Using `github.com/go-json-experiment/json` instead of the standard `encoding/json` package.

#### GoFiber

`GoFiber` is a incredibly fast framework for serving HTTP based on the `FastHTTP` package. The framework is optimized to be faster than Rust web servers, a testament to its speed. This makes it a good choice for a proxy where you want the proxy to be out of the way as much as possible.

With that said,, it certainly can be argued that we don't require the speed of `GoFiber` for this project.

It does have a lot of support in the community and a vast amount of middleware that can be used to extend the functionality of the proxy.

Because we aren't using the advanced capabilities of `GoFiber`, the standard `net/http` package was not easier to understand or use. And while there are other frameworks built around `net/http` like `Gin`, they are not as fast as `GoFiber` by orders of magnitude.

There might be good reason to try and utilize `https://pkg.go.dev/net/http/httputil#ReverseProxy` for this project. But looking at it I didn't see that this was going to be an easier solution.

#### Variable Argument Types

The other method for doing this would be to add 3 new versions of the `Agent Baker` calls that take the `VersionedReq` type.

But it is simpler for deployment and usage to have a single endpoint that can handle all the different types of requests, both old and new.

#### Embed.FS

The `embed.FS` filesystem is a feature introduced in Go 1.16 that allows us to embed files into the binary. This is a great feature for this project because it allows us to have a single binary that contains all the versions of `Agent Baker` that we need.

The other ways to do this are less palatable, such as adding the data to a container. That decouples BB from `Agent Baker` instances when you want tight integration.

#### github.com/go-json-experiment/json

This is a new JSON package that was introduced at the last GopherCon. It is a `mostly` drop in replacement for the standard `encoding/json` package. The main difference is that it is faster and has a few more features.

It was written by a former member of the Go team and is mostly what I think the `encoding/json` package will look like in the new `v2` version.

### Future Optimizations

The code should be pretty fast and use very little memory. However, there are a few things that could be done to optimize the code further:

- Use of `sync.Pool` for several `struct` types.
- Removal of `reflect.ValueOf(<type>).IsZero()` in favor of a `nil` check or just directly checking the values. Favor the second over the first to avoid GC.
- We marshal and unmarshal the JSON data twice. If we directly read the body and look for a prefix of `ABVersion` we could simply extract the version directly and content from the body with a single pass. This would save us from memory allocation and marshal/unmarshal time.
- Move from `IP` connections to `unix` sockets.

At this point, these are all are micro-optimization that are probably not worth doing simply because the bottleneck is going to be `Agent Baker` and not the proxy.
