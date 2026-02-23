package fiface

import (
	"context"
	"fmt"
)

func ExampleNew() {
	ctx := context.Background()

	// This consumer uses the default implememntation
	cons := New("some-consumer", Config{
		ConsumeAlpha: func(ctx context.Context, alpha Alpha) error {
			fmt.Println("handle an alpha")
			return nil
		},
	})

	// This prints.
	cons.ConsumeAlpha(ctx, 10)

	// This safely does nothing.
	cons.ConsumeBeta(ctx, 12.3)

	// Output: handle an alpha
	// default beta
}
