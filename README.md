# redis-lock

[![Build Status](https://travis-ci.org/linxGnu/redis-lock.png?branch=master)](https://travis-ci.org/linxGnu/redis-lock)
[![GoDoc](https://godoc.org/github.com/linxGnu/redis-lock?status.png)](http://godoc.org/github.com/linxGnu/redis-lock)
[![Go Report Card](https://goreportcard.com/badge/github.com/linxGnu/redis-lock)](https://goreportcard.com/report/github.com/linxGnu/redis-lock)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

Forked from [redis-lock](https://github.com/bsm/redis-lock)

Simplified distributed locking implementation using [Redis](http://redis.io/topics/distlock). 

Different from upstream:
- Redis-Locker from upstream is not safe for concurrent use. When 02 routines (within a process) access the same token, token is refresh (extend), not `locked`. Redis-Locker becomes useless, these routines both granted the lock at the same time.

My fork changes the behaviour to be likely as sync.Mutex. Combine with nature of redis-locker, no 02 routines/processes/machines can be granted at the same time. 

For usecase, please see examples.

## Examples

### Using locker.Obtain

```go
import (
  "fmt"
  "time"

  "github.com/linxGnu/redis-lock"
  "github.com/go-redis/redis"
)

func main() {
	// Connect to Redis
	client := redis.NewClient(&redis.Options{
		Network:	"tcp",
		Addr:		"127.0.0.1:6379",
	})
	defer client.Close()

	// Obtain a new lock with default settings
	locker, err := lock.Obtain(client, "lock.foo", nil)
	if err != nil {
		fmt.Printf("ERROR: %s\n", err.Error())
		return
	} else if locker == nil {
		fmt.Println("ERROR: could not obtain lock")
		return
	}

	// Don't forget to unlock in the end
	defer locker.Unlock()

	// Run something
	fmt.Println("I have a lock!")
	time.Sleep(200 * time.Millisecond)
}
```

### Error handling

```go
import (
  "fmt"
  "time"

  "github.com/linxGnu/redis-lock"
  "github.com/go-redis/redis"
)

func main() {
	// Connect to Redis
	client := redis.NewClient(&redis.Options{
		Network: "tcp",
		Addr:    "127.0.0.1:6379",
	})
	defer client.Close()

	// Create a new lock with default settings
	locker := lock.New(client, "lock.foo", nil)

	// warmup connection to redis
	if err := locker.Lock(); err == nil {
		locker.Unlock()
	}

	// Run something
	fmt.Println("I have another lock!")
	time.Sleep(200 * time.Millisecond)
}
```

## Documentation

Full documentation is available on [GoDoc](http://godoc.org/github.com/linxGnu/redis-lock)

## Testing

Simply run:

    make


