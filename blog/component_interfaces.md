# Functional interface pattern in Golang

The [`http.HandlerFunc`](https://pkg.go.dev/net/http#HandlerFunc) type
is a function type that implements a single interface method.

```go
// A Handler responds to an HTTP request.
type Handler interface {
	ServeHTTP(ResponseWriter, *Request)
}

// HandlerFunc(f) is a [Handler] that calls f.
type HandlerFunc func(ResponseWriter, *Request)

// ServeHTTP calls f(w, r).
func (f HandlerFunc) ServeHTTP(w ResponseWriter, r *Request) { 
    f(w, r) 
}
```

This device makes it easy to build an HTTP service from a function
literal. Embed this type in a struct and it becomes part of an HTTP
service.

This blog post explores how the "functional interface" pattern works
in Go and how it can help us maintain safe and stable interfaces, in
particular when we want to leave room for optional functionality in
the future.

# What problem does this address?

Module compatability has been challenge for Go developers since the
module system was introduced. Over time, we've come to understand what
we can and can't do, when it comes to evolving our software. Today, we
have tools such as `gorelease` that can warn developers when they make
changes that break module compatibility.

When a major Go library makes a module-incompatible change, it creates
a problem across the ecosystem for other libraries that depend on
it. As soon as an incompatible release appears, the `go get` tool is
likely to find and apply it.

When a common library with an incompatible release is first picked up
by other common libraries, then the problem begins. At that point,
users that depend on both libraries are forced to address the
incompatibility before they can update any of their common libraries.

This is the Go module system serving its function, which rests on
assertions about semantic version numbers and module
compatibility. The problem is, incompatible changes in a common
library have a wide impact. An earlier post on this blog covered
[guidelines for module
compatibility](https://go.dev/blog/module-compatibility), explaining
the challenge and listing many best practices.

For the module system to work correctly, we must not make breaking
changes unless without a new major version number. Changing the major
version, however, can leave users behind. When we follow the rules for
module compatibility without changing the major version number, we
make it easy to upgrade our dependencies.

As a security concern, we should always be able to upgrade a
dependency quickly. A security incident combined with an incompatible
change puts users into a difficult emergency.

# Illustrated


The gRPC-Go team will break experimental or deprecated interfaces to
avoid changing the major version [**as a
policy**](https://github.com/grpc/grpc-go/blob/master/Documentation/versioning.md#versioning-policy).

This policy has a terrible impact on the Go user community! By
transitivity, if any of your dependencies update to the latest gRPC,
and if for any reason you must update your dependency, you are
immediately forced to confront gRPC-Go breakage.
 we
How could the gRPC-Go team do this better? The Go team's guidelines do
not go far enough. I'll explain.


I first saw this problem working with the OpenTelemetry Collector,
which has both component interfaces and a build process for creating
binaries from a custom list of components. This is to let users (and
vendors) adopt the OpenTelemetry Collector and build it with private
components. As an example, the [Caspar Water
Collector](https://github.com/jmacd/caspar.water/blob/dcd1f85c9c8b7b9c96e3b6ce85a6232439c30bee/collector/build.yaml#L3)
lists over ten custom components (with an IIoT theme).

The way this works, a new `go.mod` file is generated from the list of
components and core libraries, and then `go mod tidy` is run to
resolve dependencies. In practice, this means you always get the
newest release of gRPC.

As an example, the [gRPC-Go 1.72.0 release
notes](https://github.com/grpc/grpc-go/releases/tag/v1.72.0) reads:

> resolver: add experimental AddressMapV2 with generics to ultimately
> replace AddressMap. Deprecate AddressMap for deletion (#8187)

Translated: If you are using the resolver pre-1.72.x `AddressMap`
interface, you should plan for a breaking change as soon as (a)
gRPC-Go follows through and removes the interface and (b) any of your
dependencies upgrades. Now, imagine how you feel when this happens
every other month.

# Working with functions

The Go team's module compatibility guidelines include recommendations
for functions:

> Instead of changing a function’s signature, add a new function.

In particular, how to plan for adding new options to existing
functions:

> If you anticipate that a function may need more arguments in the
> future, you can plan ahead by making optional arguments a part of
> the function’s signature.

They describe the [functional option pattern](https://commandcenter.blogspot.com/2014/01/self-referential-functions-and-design.html),

> Another way to provide new options in the future is the “Option
> types” pattern, where options are passed as variadic arguments.

They suggest using structs (e.g., a `*Config` struct) as the simplest
approach, noting that both are responsible choices.

# Working with structs

Structs are relatively easy to work with, in terms of future
compatibility, because of `nil` semantics and zero initialization. If
you want to add a field in the future, just ensure that the zero state
is a good default, usually meaning a feature is disabled.

> When adding a field, make sure that its zero value is meaningful and
> preserves the old behavior

With a few precautions, it is safe to add fields to structs to
introduce new options.

> If you have an exported struct type, you can almost always add a
> field or remove an unexported field without breaking compatibility.

In the Go-team guidelines, they demonstrate adding a function-valued
field,

```go
type ListenConfig struct {
    Control func(network, address string, c syscall.RawConn) error
}
```

In this case the `nil` value means no user-supplied control function.
The "almost always" quoted above is about whether structs are
comparable,

> If all the field types in a struct are comparable ... adding a new
> field of uncomparable type will ... breaking any code that compares
> values of that struct type.

They also recommend making structs non-comparable if you plan to add
functional configuration in the future,

> To prevent comparison in the first place, make sure the struct has a
> non-comparable field.

This can be done in a few ways, for example,

```go
// Config is initially empty. We can add new options in the future.
type Config struct {
    _ [0]func()
}
```

As a downside, the `struct`-based approach to future compatibility
requires callers to test for `nil` before use, otherwise their program
panics.

# Working with interfaces

Adding a method to an exported interface is a breaking change. The
first recommended approach is,

> define a new interface with the new method, and then wherever the
> old interface is used, dynamically check whether the provided type
> is the older type or the newer type.

Ignoring runtime cost, this dynamic approach has limits,

> This strategy only works when the old interface without the new
> method can still be supported, limiting the future extensibility of
> your module.

If you can avoid this, you should.

> Where possible, it is better to avoid this class of problem
> entirely. When designing constructors, for example, prefer to return
> concrete types.

A final guideline describes the _sealed interface_ pattern in Go,

> if you do need to use an interface but don’t intend for users to
> implement it, you can add an unexported method.

In this case, when users consume the interface but are not permitted
to implement it, then it is safe to add interface methods without
breaking module compatibility.

```go
type Public interface {
    // A public method.
    Method()

    // Users can't implement this directly.
    sealed()
}
```

At this point, we have every tool we need.

# Functional interfaces

In Java, the term _functional interface_ refers to an interface with
exactly one abstract method. A well known example is
`java.lang.Runnable`, with its `run()` method. Java 8 introduced the
ability to define functional interfaces using lambda syntax,

```java
Runnable runnable = () -> {
    System.out.println("Lambda Runnable running in a new thread.");
};
```

We can do the same in Go with the use of function types.

As a model, we'll use an interface similar in spirit to an
OpenTelemetry Collector component, a consumer of telemetry data with
different data types for each signal. Here, the `Alpha` and `Beta`
represent different signals that a component can consume. Suppose we
are planning to add a new signal `Gamma` in the future.

```go
// Consumes different kinds of things. 
type Consumer interface {
	ConsumeAlpha(context.Context, Alpha) error
	ConsumeBeta(context.Context, Beta) error

	// Users can't implement this directly. Use New().
	sealed()
}
```

For each method in the interface, define a function type named
after the corresponding method with a "Func" suffix:

```go
// Single abstract method for ConsumeAlpha.
type ConsumeAlphaFunc func(context.Context, Alpha) error
```

Now, give the function an implementation of the corresponding method
and make sure to handle the `nil` case.

```go
// Functional interface for ConsumeAlpha.
func (f ConsumeAlphaFunc) ConsumeAlpha(ctx context.Context, alpha Alpha) error {
    if f == nil {
        // Default behavior (e.g., this signal is not implemented).
        return UnimplementedErr
    }
    return f(ctx, alpha)
}
```

Treating the `nil` case is an important safety mechanism, it also
follows from the "zero value is meaningful and preserves the old
behavior" guideline. 

To evolve multi-method interfaces, we will use a composition of
functional interface types. By sealing the interface and using
nil-safe function types to configure new, optional behavior, the
interface compatibility problem can be solved using the guidelines for
functions and structs that we already know.

Since the interface is sealed, you control the constructor. If you did
not plan for extensibility, you may need to introduce a new
constructor function for extensibility. Hopefully, you planned for
extensibility using either of the approaches described by the Go team
(i.e., pointer-to-config-struct or functional-options, either one is a
reasonable choice). Using the simpler approach, we might use this
constructor signature:

```go
// Create a new consumer.
func New(name string, config Config) Consumer { ... }
```

Now, a constructor call like:

```go
consumer.New("some-consumer", consumer.Config{
  ConsumeAlphaFunc: func(ctx context.Context, alpha Alpha) error {
    // Handle the Alpha signal.
    return nil
  }),
  ConsumeBetaFunc: func(ctx context.Context, beta Beta) error {
    // Handle the Beta signal.
    return nil
  }),
})
```

Or, we could use the functional option pattern:

```go
consumer.New("some-consumer",
    consumer.WithAlpha(func(ctx context.Context, alpha Alpha) error {
        // Handle the Alpha signal.
        return nil
    }),
    consumer.WithBeta(func(ctx context.Context, beta Beta) error {
        // Handle the Beta signal.
        return nil
    }),
})
```

# Implementing the functional interface pattern

There are more than one ways to implement it. Here's an example
implementation:

```go
// Configure the consumer interface.
type Config {
    // How to consume an Alpha.
    ConsumeAlpha ConsumeAlphaFunc

    // How to consume a Beta.
    ConsumeBeta ConsumeBetaFunc
}

// Implementation of the sealed interface.
type consumerImpl struct {
    name string // and other details

    ConsumeAlphaFunc // implements the ConsumeAlpha method
    ConsumeBetaFunc  // implements the ConsumeBeta method
}

// This is a sealed interface.
func (consumerImpl) sealed() { }

// Test the interface.
var _ Consumer = &consumerImpl{}

// Create a new consumer.
func New(name string, config Config) Consumer {
	return &consumerImpl{
		name:             name,
		ConsumeAlphaFunc: config.ConsumeAlpha,
		ConsumeBetaFunc:  config.ConsumeBeta,
	}
}
```

The use of a pointer implementation (i.e., `&consumerImpl{...}) in
this case is a matter of safety, because it makes the interface value
comparable.

# Extension points

In the OpenTelemetry Collector, we have a document wrapping all of all
of these patterns that we call our [component interface guidelines
(wip)](https://github.com/open-telemetry/opentelemetry-collector/pull/14532),
featuring the practices  discussed:

- Extensive use of the functional interface pattern
- Functional options pattern used in top-level interfaces
- Sealed interfaces that we can extend safely and easily.

@@@ we have our own legacy, of course; we prefer to keep unstable
things in v0 modules, however we made a few mistakes, the guidelines
are to help prevent more mistakes from happening in the future.

In case of v1+ modules, which are those we have declared stable,
unsealed interfaces come with a promise of no breaking changes,
meaning not without a major version change. While many of the core
interfaces are sealed, extension interfaces are an important
exception. These are stable interfaces that serve as public extension
points.

As an example, the Collector has [`extensionmiddleware`]() interfaces
for implementing RPC-specific kinds of middleware. Here, we support
gRPC and HTTP protocols, with client- and server-specific cases, a
total of four separate unsealed interfaces. Middleware extension
components implement some or all of these interfaces. Receiver and
exporter components generally use one of these, configured on their
behalf through our `configgrpc` and `confighttp` packages.

These interfaces will be supported at least throughout v1, and even
though they are not sealed, we still follow the functional interface
pattern.  This means for every public interface method name,
`<Method>Func` type matching every public interface method by name,
with an appropriate behavior for `nil`.

This brings us back to `ServeHTTP`, which does exactly what it says,
calls `f(w, r)`. If the `HandlerFunc` is nil, the goroutine will
panic. A more graceful default behavior would be to write HTTP status
code 501 (not implemented).

```
// ServeHTTP calls f(w, r).
//
// If f is nil, calls w.WriteHeader(StatusNotImplemented).
func (f HandlerFunc) ServeHTTP(w ResponseWriter, r *Request) {
    if f == nil {
        w.WriteHeader(StatusNotImplemented)
        return
    }
	f(w, r)
}
```

# Patterns in Golang

If you haven't seen it before, the **functional interface** pattern
described above can be a real help in Go, a safe way to add optional
functionality without changing major versions.

We've covered a few other patterns that help us manage our commitment
to module compatibility:

- Functional options
- Sealed interfaces
- Incomparable structs.

We've reiterated [guidelines for module compatibility posted on the Go
Blog](https://go.dev/blog/module-compatibility), the foundation of our
approach.

1. The zero value for every field and function must be meaningful, with good default behavior
2. Do not change function signatures, add new functions instead
3. Plan for the future, either by option structs or the functional option pattern
4. Make your structs incomparable at first, otherwise remain comparable
5. Do not add methods to an interface, do not remove an interface
6. Seal the interfaces that you plan to extend, provide constructor functions
7. Use the functional interface pattern with optional functions, for consistency

Library maintainers with a public-facing interfaces in Go, including
many OpenTelemetry maintainers, are required to follow these rules if
they want to avoid breaking the users that depend on them.

If you can't follow these rules, release new major versions, otherwise
you're planning to break your users, and we don't want that. We think
these patterns will help!
