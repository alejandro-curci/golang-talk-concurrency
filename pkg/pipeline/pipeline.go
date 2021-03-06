package pipeline

import (
	"context"
	"fmt"
	"sync"
)

// A pipeline is a series of stages connected by channels,
// where each stage is a group of goroutines running the same function.

// In each stage, the goroutines:
// 1- receive values from upstream via inbound channels
// 2- perform some function on that data, producing new values
// 3- send those values downstream via outbound channels

// first stage -> source or producer
// last stage -> sink or consumer

// generate is the first stage, it converts a list of integers into a channel which emits those numbers
func generate(ctx context.Context, numbers ...int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out) // DEFER CLOSING
		for _, n := range numbers {
			select { // SELECT STATEMENT
			case out <- n:
			case <-ctx.Done(): // LISTEN TO CONTEXT CHANNEL
				return // EARLY RETURN
			}
		}
	}()
	return out
}

// power is the second stage, it powers the numbers received from stage 1 and sends them to another channel
func power(ctx context.Context, in <-chan int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out) // DEFER CLOSING
		for n := range in {
			select { // ADD SELECT STATEMENT
			case out <- n * n:
			case <-ctx.Done(): // LISTEN TO CONTEXT CHANNEL
				return // EARLY RETURN
			}
		}
	}()
	return out
}

// sum is the third stage, it sums all the numbers received from the previous stage
func sum(in <-chan int) <-chan int {
	out := make(chan int)
	go func() {
		var total int
		for n := range in {
			total += n
		}
		out <- total
		close(out)
	}()
	return out
}

// FAN-OUT
// Multiple functions can read from the same channel until that channel is closed.
// It provides a way to distribute work amongst a group of workers to parallelize CPU use and I/O.

// FAN-IN
// A function can read from multiple inputs and proceed until all are closed by multiplexing
// the input channels onto a single channel that’s closed when all the inputs are closed.

func merge(ctx context.Context, channels ...<-chan int) <-chan int {
	var wg sync.WaitGroup
	out := make(chan int)

	// closure -> sends values from channels into the out channel
	send := func(ch <-chan int) {
		defer wg.Done() // DEFER CLOSING
		for n := range ch {
			select { // SELECT STATEMENT
			case out <- n:
			case <-ctx.Done(): // LISTEN TO CONTEXT CHANNEL
				return // EARLY RETURN
			}
		}
	}

	wg.Add(len(channels))
	for _, ch := range channels {
		go send(ch)
	}

	// wait and close the out channel in a different goroutine
	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

// CANCELLATION
// In real pipelines, stages don’t always receive all the inbound values (no need to wait for them or an error occurred).
// If a stage fails to consume all the inbound values, the goroutines attempting to send those values will block indefinitely,
// causing a resource leak (goroutines are not garbage collected, they must exit on their own)
// We need to provide a way for downstream stages to indicate to the senders that they will stop accepting input.

// 1) USING EMPTY STRUCT TO MANUALLY SIGNAL THE CANCELLATION
// problem = each downstream receiver needs to know the number of potentially blocked upstream senders

// 2) CLOSING THE CHANNEL
// Remember!!!! => a receive operation on a closed channel can always proceed immediately,
// yielding the element type’s zero value

// 3) USE CONTEXT PACKAGE INSTEAD OF THE DONE CHANNEL
// same functionality, more elegant

func main() {
	ctx := context.Background()            // CREATE A CONTEXT
	ctx, cancel := context.WithCancel(ctx) // CANCEL FUNCTIONALITY
	defer cancel() // DEFER CANCELLATION

	in := generate(ctx, 15, 2, 9, 23, 91)

	ch1 := power(ctx, in)
	ch2 := power(ctx, in)

	out := merge(ctx, ch1, ch2)

	for i := 0; i < 3; i++ {
		fmt.Println(<-out)
	}
}
