# Functional interface pattern in Golang

Nearly every Go programmer knows the [functional option
pattern](https://commandcenter.blogspot.com/2014/01/self-referential-functions-and-design.html).

Most Go programmers knows about
[`http.HandlerFunc`](https://pkg.go.dev/net/http#HandlerFunc). In
Java, there is a similar pattern and Go programmers ought to know this
by name, the _functional interface_ pattern.

# What problem does this solve?

Module compatability is a problem in Golang, especially when it comes
to interfaces. The Go team has written [guidelines for module
compatibility](https://go.dev/blog/module-compatibility) explaining
the challenge and listing many best practices. 

> it is usually better to change your existing package in a compatible way

For the module system to work correctly, you must not make breaking
changes unless you increment the major version. Changing the major
version, however, can leave users behind.

> breaking changes to a v1+ module must happen as part of a major version bump

If you've worked in Go long enough, you've probably worked with the
gRPC-Go library. If you're lucky, you've never experienced one of
their breaking changes, but it's a fact of life for many Go
developers. The gRPC-Go team will break experimental or deprecated
interfaces to avoid changing the major version [**as a
policy**](https://github.com/grpc/grpc-go/blob/master/Documentation/versioning.md#versioning-policy).

This policy has a terrible impact on the Go user community! By
transitivity, if any of your dependencies update to the latest gRPC,
and if for any reason you must update your dependency, you are
immediately forced to confront gRPC-Go breakage.

How could the gRPC-Go team do this better? The Go team's guidelines do
not go far enough. I'll explain.

# Illustrated

I first saw this problem working with the OpenTelemetry Collector,
which has both component interfaces and a build process for creating
binaries from a custom list of components. This is to let users (and
vendors) adopt the OpenTelemetry Collector and build it with private
components. As an example, the [Caspar Water
Collector](https://github.com/jmacd/caspar.water/blob/dcd1f85c9c8b7b9c96e3b6ce85a6232439c30bee/collector/build.yaml#L3)
lists over ten custom components (with an IIoT theme).

The way this works, a new `go.mod` file is generated from the list of
components and core libraries, and then `go mod tidy` was run to
resolve dependencies. In practice, this means you always get the
newest release of gRPC. Eventually, we added an option to build with
an external `go.mod`, alleviating the problem somewhat, but until then
every gRPC-Go breaking change led to a fire drill.

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

The Go team is opinionated over the _functional option_ pattern,

> Another way to provide new options in the future is the “Option
> types” pattern, where options are passed as variadic arguments.

and suggests using structs (e.g., a `*Config` struct) as the simpler
approach, noting reasons the functional option pattern is not always
best.

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

Plan to support public interfaces indefinitely, in other words. If you
can't do that, plan to release a new major version for modifying
interfaces. If you can't do that, tell your users your plans include a
potential to break their build.

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
    private()
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

We can do the same in Go.

As a model, we'll use an interface similar in spirit to an
OpenTelemetry Collector component, a consumer of telemetry data with
different data types for each signal. Here, the `A`, `B`, and `C`
represent different different signals that a component can consume,
and we are planning to add a new signal `D`.

```go
type Consumer interface {
    // A component consumes different kinds of things. 

    ConsumeA(A)
    ConsumeB(B)
    ConsumeC(C)

    // Users can't implement this directly.
    private()
}
```









# Patterns in Golang

Why are there so few named patterns in Go?

1. Functional options
2. Functional interfaces
3. Sealed interfaces
4. Non-comparable types








```go
type Handler interface {
    ServeHTTP(ResponseWriter, *Request)
}

type HandlerFunc func(ResponseWriter, *Request)

func (f HandlerFunc) ServeHTTP(w ResponseWriter, r *Request) {
    f(w, r)
}
```

A function type that implements an interface. You've seen this. 
You've used it a hundred times. But have you ever extended it?

I've been working on the OpenTelemetry Collector for the past few
years, trying to add rate-limiter and middleware extension APIs.
Along the way I discovered that the Collector uses a generalized
version of this pattern across its core interfaces -- and that
nobody had written it down. After several failed attempts at
documenting it as an RFC
([#13263](https://github.com/open-telemetry/opentelemetry-collector/pull/13263),
[#13902](https://github.com/open-telemetry/opentelemetry-collector/pull/13902)),
I finally have something the maintainers are
[accepting](https://github.com/open-telemetry/opentelemetry-collector/pull/14532):
"Component Interface Guidelines."

The pattern doesn't have a catchy name yet. I've called it
"Functional Composition," "Functional Interface," and now
"Component Interface Guidelines." The name isn't important. What
matters is that it solves a real problem in Go: **how to evolve
interfaces without breaking consumers.**

## The problem

Go interfaces are brittle. Add a method to an exported interface,
and every external implementation breaks. The standard advice from
the Go blog is: [provide constructor functions instead of
expecting users to implement
interfaces](https://go.dev/blog/module-compatibility#working-with-interfaces).

But the Collector has dozens of extension points. Extensions
*must* implement interfaces. Factory types *must* be constructed
by external packages. How do you square this circle?

## The pattern

For every method in a public interface, declare a corresponding
function type with the same signature. The function type
implements the interface method, and **checks for nil to provide
no-op behavior**.

The Collector's HTTP middleware interface is the clearest example.
It lives in
[`extensionmiddleware`](https://github.com/open-telemetry/opentelemetry-collector/blob/main/extension/extensionmiddleware/client.go),
and if you know `http.HandlerFunc` you already understand it.

### The interface

```go
// HTTPClient is an interface for HTTP client middleware extensions.
type HTTPClient interface {
    GetHTTPRoundTripper(base http.RoundTripper) (http.RoundTripper, error)
}
```

One method. It takes a base `RoundTripper` and returns a wrapped
one -- the same shape as HTTP server middleware, but for outbound
calls.

### The function type

```go
type GetHTTPRoundTripperFunc func(base http.RoundTripper) (http.RoundTripper, error)

func (f GetHTTPRoundTripperFunc) GetHTTPRoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
    if f == nil {
        return base, nil
    }
    return f(base)
}
```

That's the whole trick. A function type that implements the
interface, with a nil check that returns a sensible default. A nil
`GetHTTPRoundTripperFunc` passes the base `RoundTripper` through
unchanged -- a perfect no-op.

Compare this with `http.HandlerFunc` from the standard library:

```go
func (f HandlerFunc) ServeHTTP(w ResponseWriter, r *Request) {
    f(w, r) // panics if f is nil
}
```

No nil check. If you pass a nil `HandlerFunc`, you crash.

### Using it as a building block

Here is the server-side equivalent:

```go
type HTTPServer interface {
    GetHTTPHandler(base http.Handler) (http.Handler, error)
}

type GetHTTPHandlerFunc func(base http.Handler) (http.Handler, error)

func (f GetHTTPHandlerFunc) GetHTTPHandler(base http.Handler) (http.Handler, error) {
    if f == nil {
        return base, nil
    }
    return f(base)
}
```

Same shape. And there are gRPC variants too:

```go
type GRPCClient interface {
    GetGRPCClientOptions() ([]grpc.DialOption, error)
}

type GetGRPCClientOptionsFunc func() ([]grpc.DialOption, error)

func (f GetGRPCClientOptionsFunc) GetGRPCClientOptions() ([]grpc.DialOption, error) {
    if f == nil {
        return nil, nil
    }
    return f()
}
```

Four interfaces, four function types, all following the same
mechanical rule: `<Method>Func` has the same signature as the
method, implements the method, and checks for nil.

### Composition through embedding

Now the real leverage. A middleware extension needs to implement
*all four* of these interfaces. In the Collector's test helpers,
that looks like this:

```go
type baseExtension struct {
    component.StartFunc
    component.ShutdownFunc
    extensionmiddleware.GetHTTPHandlerFunc
    extensionmiddleware.GetGRPCServerOptionsFunc
    extensionmiddleware.GetHTTPRoundTripperFunc
    extensionmiddleware.GetGRPCClientOptionsFunc
}
```

Each embedded function type implements one interface. If the
field is nil, the no-op behavior kicks in. This gives you two
constructors for free:

```go
// NewNop returns a middleware that does nothing.
// Every method passes through or returns zero values.
func NewNop() extension.Extension {
    return &baseExtension{}
}

// NewErr returns a middleware where every method fails.
func NewErr(err error) extension.Extension {
    return &baseExtension{
        GetHTTPRoundTripperFunc: func(http.RoundTripper) (http.RoundTripper, error) {
            return nil, err
        },
        GetGRPCClientOptionsFunc: func() ([]grpc.DialOption, error) {
            return nil, err
        },
        GetHTTPHandlerFunc: func(http.Handler) (http.Handler, error) {
            return nil, err
        },
        GetGRPCServerOptionsFunc: func() ([]grpc.ServerOption, error) {
            return nil, err
        },
    }
}
```

`NewNop()` is literally `&baseExtension{}`. The zero value of
every embedded function type is nil, and nil means no-op. That's
not a coincidence; it's the point.

### Consuming through type assertions

On the consumer side, the Collector uses type assertions to
discover which capabilities an extension provides:

```go
func (m Config) GetHTTPClientRoundTripper(
    _ context.Context,
    extensions map[component.ID]component.Component,
) (func(http.RoundTripper) (http.RoundTripper, error), error) {
    if ext, found := extensions[m.ID]; found {
        if client, ok := ext.(extensionmiddleware.HTTPClient); ok {
            return client.GetHTTPRoundTripper, nil
        }
        return nil, errNotHTTPClient
    }
    return nil, fmt.Errorf("failed to resolve middleware %q: %w", m.ID, errMiddlewareNotFound)
}
```

An extension can implement `HTTPClient`, or `GRPCServer`, or
both, or neither. The consumer checks at runtime. This is how
the pattern scales to dozens of extension points without a
combinatorial explosion of interface sets.

## The rate-limiter example

The middleware example is simple because each interface has a
single method. The same pattern works for multi-method
interfaces. Here's a sketch from the [rate-limiter extension
draft](https://github.com/open-telemetry/opentelemetry-collector/pull/13241):

```go
// RateReservation is returned when a caller reserves throughput.
type RateReservation interface {
    WaitTime() time.Duration
    Cancel()
}

type WaitTimeFunc func() time.Duration

func (f WaitTimeFunc) WaitTime() time.Duration {
    if f == nil {
        return 0
    }
    return f()
}

type CancelFunc func()

func (f CancelFunc) Cancel() {
    if f == nil {
        return
    }
    f()
}

type rateReservationImpl struct {
    WaitTimeFunc
    CancelFunc
}

func NewRateReservation(wt WaitTimeFunc, c CancelFunc) RateReservation {
    return rateReservationImpl{WaitTimeFunc: wt, CancelFunc: c}
}
```

A nil `WaitTimeFunc` means "don't wait." A nil `CancelFunc` means
"nothing to cancel." A `NewRateReservation(nil, nil)` is a valid
no-op reservation.

The limiter itself follows the same shape:

```go
type RateLimiter interface {
    ReserveRate(ctx context.Context, weight int) (RateReservation, error)
}

type ReserveRateFunc func(context.Context, int) (RateReservation, error)

func (f ReserveRateFunc) ReserveRate(ctx context.Context, weight int) (RateReservation, error) {
    if f == nil {
        return NewRateReservation(nil, nil), nil
    }
    return f(ctx, weight)
}

type rateLimiterImpl struct {
    ReserveRateFunc
}

func NewRateLimiter(f ReserveRateFunc) RateLimiter {
    return rateLimiterImpl{ReserveRateFunc: f}
}
```

Now you can build a real limiter by wrapping `golang.org/x/time/rate`:

```go
func NewTokenBucketLimiter(rps float64, burst int) RateLimiter {
    limiter := rate.NewLimiter(rate.Limit(rps), burst)

    return NewRateLimiter(func(ctx context.Context, weight int) (RateReservation, error) {
        rsv := limiter.ReserveN(time.Now(), weight)
        if !rsv.OK() {
            return nil, errors.New("rate limit exceeded")
        }
        return NewRateReservation(
            func() time.Duration { return rsv.DelayFrom(time.Now()) },
            rsv.Cancel,
        ), nil
    })
}
```

Or add logging as a decorator:

```go
func WithLogging(limiter RateLimiter, logger *slog.Logger) RateLimiter {
    return NewRateLimiter(func(ctx context.Context, weight int) (RateReservation, error) {
        logger.Info("rate limit check", "weight", weight)
        return limiter.ReserveRate(ctx, weight)
    })
}
```

## Why this works

### Sealed interfaces evolve safely

Add a `private()` method to prevent external implementations:

```go
type RateLimiter interface {
    ReserveRate(ctx context.Context, weight int) (RateReservation, error)
    private()
}

type rateLimiterImpl struct {
    ReserveRateFunc
}

func (rateLimiterImpl) private() {}
```

External consumers can't implement `RateLimiter` directly -- they
must use `NewRateLimiter`. When you add a new method and its
corresponding `<Method>Func`, the existing constructors still
compile. They gain the new method's no-op behavior
automatically, because uninitialized function types return zero
values.

### Open interfaces enable capability detection

Not every interface should be sealed. The middleware interfaces
are open -- external implementations are the whole purpose.
They evolve by adding *companion interfaces* instead of methods:

```go
// Original -- remains unchanged forever.
type GRPCClient interface {
    GetGRPCClientOptions() ([]grpc.DialOption, error)
}

// Added later when we realized some options need context.
type GRPCClientContext interface {
    GetGRPCClientOptionsContext(context.Context) ([]grpc.DialOption, error)
}
```

Consumers use type assertions to check for the new capability.
Old implementations continue to work.

### The nil-safe HandlerFunc

`http.HandlerFunc` is the prototype. But Go's standard library
stopped short:

- `HandlerFunc` doesn't check for nil
- `http.RoundTripper` doesn't have a `RoundTripperFunc`
- `Handler.ServeHTTP` takes `*http.Request`, a concrete type

This pattern fixes all three. Every function type is nil-safe.
Every method gets a `Func` type. Every parameter is an interface.
These constraints together enable an approach to safe
interface evolution that the standard library doesn't provide.

## References

- [Component Interface Guidelines (PR #14532)](https://github.com/open-telemetry/opentelemetry-collector/pull/14532)
- [RFC: Functional Interface pattern (PR #13902)](https://github.com/open-telemetry/opentelemetry-collector/pull/13902)
- [Rate-limiter extension draft (PR #13241)](https://github.com/open-telemetry/opentelemetry-collector/pull/13241)
- [Go Blog: Working with Interfaces](https://go.dev/blog/module-compatibility#working-with-interfaces)
- [http.HandlerFunc](https://pkg.go.dev/net/http#HandlerFunc)
- [Functional Options (Rob Pike)](https://commandcenter.blogspot.com/2014/01/self-referential-functions-and-design.html)

## Related patterns in other languages

This pattern combines several well-known ideas under one roof.

**Functional Interface / SAM type (Java).** Java 8 formalized
the "Single Abstract Method" interface with `@FunctionalInterface`,
letting a lambda implement a one-method interface. Each
`<Method>Func` type here is a SAM type. But Java's version
doesn't have nil-safe defaults.

**Null Object (GoF).** The nil-checking behavior is textbook
Null Object. `NewNop()` returning `&baseExtension{}` is a Null
Object constructor. The classic pattern uses a dedicated subclass;
this one uses the zero value of function types, which is more
compact.

**Default methods.** The *problem* — evolving interfaces without
breaking implementors — is solved at the language level elsewhere:
Java has `default` methods in interfaces (added in Java 8 for
exactly this reason), Rust traits can provide default
implementations, Swift has protocol extensions, Kotlin interface
methods can have bodies, and Haskell type class methods can have
defaults. Go has none of these, so the nil check on the function
type is a manual encoding of what those languages provide as a
first-class feature.

**Strategy (GoF).** The interchangeable function values are
strategies. `ReserveRateFunc` is a rate-limiting strategy. But
"Strategy" doesn't capture the nil-safety or the
composition-through-embedding aspect.

No single name covers all of this. The closest is probably
**nil-safe functional interface** — it's the nil check that
distinguishes the pattern from `http.HandlerFunc` and from
Java's `@FunctionalInterface`. The composition through embedding
and the sealed/open evolution rules are specific to Go's type
system: structural typing, unexported methods, and nil-safe method
dispatch on function types. No other language needs exactly this
pattern because no other language has exactly this set of
constraints.
