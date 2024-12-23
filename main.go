package main

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	streamName    = "expiration_events"
	consumerGroup = "expiration_handlers"
	numConsumers  = 64
	totalKeys     = 3000000
	keyExpiration = 100 * time.Millisecond
	processedKeys = "processed_keys"   // Set to track processed keys
	maxWaitTime   = 3600 * time.Second // Increased to 1 hour to account for slower key creation
)

var (
	rdb              *redis.Client
	keyCounter       uint64
	consumerStats    = make([]uint64, numConsumers)
	duplicateCount   uint64
	skippedCount     uint64
	currentUnexpired int64 // Track current number of unexpired keys (using signed int for atomic add/sub)
	maxUnexpiredKeys int64 // Track maximum number of unexpired keys seen
)

func main() {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), maxWaitTime)
	defer cancel()

	rdb = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer rdb.Close()

	// Clear ALL existing keys and streams
	rdb.FlushAll(ctx)

	// Enable keyspace notifications for expired events
	rdb.ConfigSet(ctx, "notify-keyspace-events", "Ex")

	// Initialize stream with a dummy message if it doesn't exist
	_, err := rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: streamName,
		Values: map[string]interface{}{"init": "true"},
	}).Result()
	if err != nil {
		fmt.Printf("Error initializing stream: %v\n", err)
		return
	}

	// Create consumer group, ignore error if it already exists
	err = rdb.XGroupCreate(ctx, streamName, consumerGroup, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		fmt.Printf("Error creating consumer group: %v\n", err)
		return
	}

	// Create error channel to propagate errors from goroutines
	errChan := make(chan error, numConsumers+1)

	// Start consumers
	var wg sync.WaitGroup
	for i := 0; i < numConsumers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if err := consumer(ctx, id); err != nil {
				errChan <- fmt.Errorf("consumer %d error: %v", id, err)
			}
		}(i)
	}

	// Start key expiration simulator
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := simulateKeyExpirations(ctx); err != nil {
			errChan <- fmt.Errorf("simulator error: %v", err)
		}
	}()

	// Wait for all keys to be processed
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	lastCount := uint64(0)
	stuckCount := 0
	maxStuckCount := 30 // 30 seconds without progress

	fmt.Println("Processing keys...")
	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\nTimeout waiting for processing to complete")
			goto DONE
		case err := <-errChan:
			fmt.Printf("\nError occurred: %v\n", err)
		case <-ticker.C:
			processedCount, err := rdb.SCard(ctx, processedKeys).Result()
			if err != nil {
				fmt.Printf("\nError getting processed count: %v\n", err)
				continue
			}

			// Check if processing is stuck
			if uint64(processedCount) == lastCount {
				stuckCount++
				if stuckCount > maxStuckCount {
					fmt.Println("\nProcessing appears to be stuck")
					goto DONE
				}
			} else {
				stuckCount = 0
				lastCount = uint64(processedCount)
			}

			elapsed := time.Since(startTime)
			rate := float64(processedCount) / elapsed.Seconds()
			current := atomic.LoadInt64(&currentUnexpired)
			max := atomic.LoadInt64(&maxUnexpiredKeys)
			fmt.Printf("\rProgress: %d/%d keys (%.1f%%) | Rate: %.0f/sec | Unexpired: %d (Max: %d)   ",
				processedCount, totalKeys,
				float64(processedCount)/float64(totalKeys)*100,
				rate,
				current,
				max)

			if processedCount == totalKeys {
				fmt.Println("\nAll keys have been processed!")
				goto DONE
			}
		}
	}

DONE:
	// Cancel context and wait for all goroutines to finish
	cancel()
	wg.Wait()
	close(errChan)

	// Print any remaining errors
	for err := range errChan {
		fmt.Printf("Error during execution: %v\n", err)
	}

	// Print final statistics
	fmt.Println("\nFinal Statistics:")
	fmt.Printf("Total keys generated: %d\n", atomic.LoadUint64(&keyCounter))
	maxUnexpired := atomic.LoadInt64(&maxUnexpiredKeys)
	fmt.Printf("Maximum number of unexpired keys at any point: %d\n", maxUnexpired)
	fmt.Printf("Current unexpired keys: %d\n", atomic.LoadInt64(&currentUnexpired))

	processedCount, err := rdb.SCard(context.Background(), processedKeys).Result()
	if err != nil {
		fmt.Printf("Error getting processed count: %v\n", err)
	} else {
		fmt.Printf("Total unique keys processed: %d\n", processedCount)
		fmt.Printf("Duplicate attempts: %d\n", atomic.LoadUint64(&duplicateCount))
		fmt.Printf("Skipped (already being processed): %d\n", atomic.LoadUint64(&skippedCount))

		if processedCount == totalKeys {
			fmt.Println("✅ All keys were processed exactly once!")
		} else {
			fmt.Printf("⚠️ Mismatch: %d keys generated but %d keys processed\n",
				totalKeys, processedCount)
		}
	}

	// Print per-consumer statistics
	fmt.Println("\nPer-Consumer Statistics:")
	total := uint64(0)
	for i := 0; i < numConsumers; i++ {
		count := atomic.LoadUint64(&consumerStats[i])
		total += count
		fmt.Printf("Consumer %d processed %d keys (%.1f%%)\n",
			i, count, float64(count)/float64(totalKeys)*100)
	}
	fmt.Printf("\nTotal events processed: %d\n", total)
}

func consumer(ctx context.Context, id int) error {
	// Add backoff mechanism for consumer
	backoff := time.Millisecond * 100
	maxBackoff := time.Second * 2

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			// Read messages with a shorter block time
			streams, err := rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    consumerGroup,
				Consumer: fmt.Sprintf("consumer-%d", id),
				Streams:  []string{streamName, ">"},
				Count:    1,
				Block:    100 * time.Millisecond,
			}).Result()

			if err != nil {
				if err == context.Canceled || err == context.DeadlineExceeded {
					return nil
				}
				if err != redis.Nil {
					fmt.Printf("Consumer %d error: %v\n", id, err)
					// Apply backoff on error
					time.Sleep(backoff)
					backoff = min(backoff*2, maxBackoff)
					continue
				}
				// Reset backoff on success
				backoff = time.Millisecond * 100
				continue
			}

			// Reset backoff on success
			backoff = time.Millisecond * 100

			for _, stream := range streams {
				for _, message := range stream.Messages {
					if err := handleMessage(ctx, id, message); err != nil {
						fmt.Printf("Consumer %d message handling error: %v\n", id, err)
						// Don't break on error, continue processing other messages
					}
				}
			}
		}
	}
}

func handleMessage(ctx context.Context, id int, message redis.XMessage) error {
	// Skip the initialization message
	if message.Values["init"] != nil {
		return rdb.XAck(ctx, streamName, consumerGroup, message.ID).Err()
	}

	// Safely get the key value
	keyVal, ok := message.Values["key"]
	if !ok || keyVal == nil {
		return rdb.XAck(ctx, streamName, consumerGroup, message.ID).Err()
	}

	key, ok := keyVal.(string)
	if !ok {
		return rdb.XAck(ctx, streamName, consumerGroup, message.ID).Err()
	}

	// First, check if key was already processed
	wasProcessed, err := rdb.SIsMember(ctx, processedKeys, key).Result()
	if err != nil {
		// Don't acknowledge message on error to allow retry
		return fmt.Errorf("error checking processed status: %v", err)
	}
	if wasProcessed {
		atomic.AddUint64(&duplicateCount, 1)
		return rdb.XAck(ctx, streamName, consumerGroup, message.ID).Err()
	}

	// Use a more unique lock key to prevent any possible collisions
	lockKey := fmt.Sprintf("lock:%s:%s", key, message.ID)
	// Increase lock timeout to prevent premature expiration
	locked, err := rdb.SetNX(ctx, lockKey, "1", 500*time.Millisecond).Result()
	if err != nil {
		// Don't acknowledge on lock error
		return fmt.Errorf("lock error: %v", err)
	}

	if !locked {
		atomic.AddUint64(&skippedCount, 1)
		// Still acknowledge as this message was handled by another consumer
		return rdb.XAck(ctx, streamName, consumerGroup, message.ID).Err()
	}

	defer func() {
		// Best effort cleanup of lock
		_ = rdb.Del(ctx, lockKey).Err()
	}()

	// Double check the key hasn't been processed while we were getting the lock
	wasProcessed, err = rdb.SIsMember(ctx, processedKeys, key).Result()
	if err != nil {
		return fmt.Errorf("error on double-check of processed status: %v", err)
	}
	if wasProcessed {
		atomic.AddUint64(&duplicateCount, 1)
		return rdb.XAck(ctx, streamName, consumerGroup, message.ID).Err()
	}

	// Process the expired key
	time.Sleep(5 * time.Millisecond)

	// Use MULTI to ensure atomic processing
	pipe := rdb.Pipeline()
	pipe.SAdd(ctx, processedKeys, key)
	pipe.Del(ctx, lockKey)

	// Execute pipeline
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("error in atomic processing: %v", err)
	}

	atomic.AddUint64(&consumerStats[id], 1)

	// Only acknowledge after successful processing
	return rdb.XAck(ctx, streamName, consumerGroup, message.ID).Err()
}

func simulateKeyExpirations(ctx context.Context) error {
	pubsub := rdb.Subscribe(ctx, "__keyevent@0__:expired")
	defer pubsub.Close()

	// Ensure subscription is ready
	if _, err := pubsub.Receive(ctx); err != nil {
		return fmt.Errorf("subscription error: %v", err)
	}

	ch := pubsub.Channel()

	// Start expiration handler in a separate goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-ch:
				if msg == nil {
					continue
				}
				atomic.AddInt64(&currentUnexpired, -1) // Decrement unexpired count
				_, err := rdb.XAdd(ctx, &redis.XAddArgs{
					Stream: streamName,
					Values: map[string]interface{}{"key": msg.Payload},
				}).Result()
				if err != nil {
					fmt.Printf("Error adding to stream: %v\n", err)
				}
			}
		}
	}()

	// Create keys with a small delay between each
	for i := uint64(1); i <= totalKeys; i++ {
		select {
		case <-ctx.Done():
			return nil
		default:
			atomic.AddUint64(&keyCounter, 1)

			// Increment current unexpired count and update max if needed
			current := atomic.AddInt64(&currentUnexpired, 1)
			for {
				max := atomic.LoadInt64(&maxUnexpiredKeys)
				if current <= max || atomic.CompareAndSwapInt64(&maxUnexpiredKeys, max, current) {
					break
				}
			}

			key := fmt.Sprintf("key:%d", i)
			err := rdb.Set(ctx, key, "value", keyExpiration).Err()
			if err != nil {
				fmt.Printf("Error setting key %s: %v\n", key, err)
			}

			time.Sleep(100 * time.Microsecond) // Add 100µs delay between key creations
		}
	}

	// Wait for all keys to be processed
	wg.Wait()
	return nil
}

// Helper function for backoff
func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
