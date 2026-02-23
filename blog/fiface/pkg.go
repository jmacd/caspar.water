package fiface

import (
	"context"
	"fmt"
)

// Alpha is ...
type Alpha int64

// Beta is ...
type Beta float64

// Consumes different kinds of things. 
type Consumer interface {
	ConsumeAlpha(context.Context, Alpha) error
	ConsumeBeta(context.Context, Beta) error

	// Users can't implement this directly. Use New().
	sealed()
}

// Single abstract method for ConsumeAlpha.
type ConsumeAlphaFunc func(context.Context, Alpha) error

// Single abstract method for ConsumeBeta.
type ConsumeBetaFunc func(context.Context, Beta) error

// Functional interface for ConsumeAlpha.
func (f ConsumeAlphaFunc) ConsumeAlpha(ctx context.Context, alpha Alpha) error {
	if f == nil {
		fmt.Println("default alpha")
		return nil
	}
	return f(ctx, alpha)
}

// Functional interface for ConsumeBeta.
func (f ConsumeBetaFunc) ConsumeBeta(ctx context.Context, beta Beta) error {
	if f == nil {
		fmt.Println("default beta")
		return nil
	}
	return f(ctx, beta)
}

// Configure the consumer interface.
type Config struct {
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
func (consumerImpl) sealed() {}

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

