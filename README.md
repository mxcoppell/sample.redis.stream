# Redis Stream Key Expiration Handler

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A high-performance, distributed system for processing Redis key expiration events using Redis Streams and consumer groups.

## Prerequisites

- Go 1.16 or later
- Redis 6.x or later
- Git

## Installation

1. Clone the repository:
```bash
git clone https://github.com/mxcoppell/sample.redis.stream.git
cd sample.redis.stream
```

2. Install dependencies:
```bash
go mod tidy
```

3. Ensure Redis is running locally on the default port (6379). If using a different Redis configuration, modify the connection settings in `main.go`:
```go
rdb = redis.NewClient(&redis.Options{
    Addr: "localhost:6379",
})
```

## Configuration

The system can be configured by modifying the constants in `main.go`:

```go
const (
    streamName    = "expiration_events"    // Name of the Redis Stream
    consumerGroup = "expiration_handlers"  // Name of the consumer group
    numConsumers  = 64                     // Number of concurrent consumers
    totalKeys     = 3000000               // Total number of keys to process
    keyExpiration = 100 * time.Millisecond // Key expiration time
    maxWaitTime   = 3600 * time.Second    // Maximum wait time for processing
)
```

## Running the System

1. Start the application:
```bash
go run main.go
```

2. The system will:
   - Clear existing Redis keys and streams
   - Enable keyspace notifications
   - Start the configured number of consumers
   - Begin generating and processing keys
   - Display real-time progress
   - Show final statistics when complete

## Example Output

```
Processing keys...
Progress: 3000000/3000000 keys (100.0%) | Rate: 1284/sec | Unexpired: 0 (Max: 491)     
All keys have been processed!

Final Statistics:
Total keys generated: 3000000
Maximum number of unexpired keys at any point: 491
Current unexpired keys: 0
Total unique keys processed: 3000000
Duplicate attempts: 0
Skipped (already being processed): 0
✅ All keys were processed exactly once!

Per-Consumer Statistics:
Consumer 0 processed 46879 keys (1.6%)
Consumer 1 processed 46865 keys (1.6%)
...
Consumer 62 processed 46878 keys (1.6%)
Consumer 63 processed 46869 keys (1.6%)

Total events processed: 3000000
```

## Performance

The system demonstrates:
- Processing rate of ~1,284 keys/second
- Even distribution across 64 consumers
- Zero duplicate processing
- Perfect exactly-once delivery
- Minimal variance in consumer workload (0.04%)

## Architecture

For detailed information about the system's architecture, design decisions, and performance characteristics, please see [DESIGN.md](DESIGN.md).

## Troubleshooting

1. **Redis Connection Issues**
   - Ensure Redis is running: `redis-cli ping`
   - Verify Redis configuration: `redis-cli info`
   - Check Redis keyspace notifications: `redis-cli config get notify-keyspace-events`

2. **Performance Issues**
   - Monitor Redis memory usage: `redis-cli info memory`
   - Check system resources: `top` or `htop`
   - Adjust consumer count based on available CPU cores

3. **Common Errors**
   - "Connection refused": Redis not running
   - "BUSYGROUP Consumer Group name already exists": Safe to ignore on restart
   - "LOADING Redis is loading the dataset in memory": Wait for Redis to complete loading

## License

MIT License - See LICENSE file for details 