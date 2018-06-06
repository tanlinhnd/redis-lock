package lock

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-redis/redis"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const testRedisKey = "__bsm_redis_lock_unit_test__"

var _ = Describe("Locker", func() {
	var (
		subject *Locker
	)
	hostname, _ := os.Hostname()
	tokenPfx := fmt.Sprintf("%s-%d-", hostname, os.Getpid())

	var newLock = func() *Locker {
		return New(redisClient, testRedisKey, &Options{
			RetryCount:  4,
			RetryDelay:  25 * time.Millisecond,
			LockTimeout: time.Second,
			TokenPrefix: tokenPfx,
		})
	}

	var getTTL = func() (time.Duration, error) {
		return redisClient.PTTL(testRedisKey).Result()
	}

	BeforeEach(func() {
		subject = newLock()
	})

	AfterEach(func() {
		Expect(redisClient.Del(testRedisKey).Err()).NotTo(HaveOccurred())
	})

	It("should normalize options", func() {
		locker := New(redisClient, testRedisKey, &Options{
			LockTimeout: -1,
			RetryCount:  -1,
			RetryDelay:  -1,
		})
		Expect(locker.opts).To(Equal(Options{
			LockTimeout: 5 * time.Second,
			RetryCount:  0,
			RetryDelay:  100 * time.Millisecond,
		}))
	})

	It("should fail obtain with error", func() {
		locker := newLock()
		locker.Lock()
		defer locker.Unlock()

		_, err := Obtain(redisClient, testRedisKey, nil)
		Expect(err).To(Equal(ErrLockNotObtained))
	})

	It("should obtain through short-cut", func() {
		Expect(Obtain(redisClient, testRedisKey, nil)).To(BeAssignableToTypeOf(subject))
	})

	It("should obtain fresh locks", func() {
		Expect(subject.Lock()).To(BeNil())

		Expect(redisClient.Get(testRedisKey).Result()).To(HaveLen(24 + len(subject.opts.TokenPrefix)))
		Expect(getTTL()).To(BeNumerically("~", time.Second, 10*time.Millisecond))

		Expect(subject.Unlock()).To(BeNil())
	})

	It("should retry if enabled", func() {
		Expect(redisClient.Set(testRedisKey, "ABCD", 0).Err()).NotTo(HaveOccurred())
		Expect(redisClient.PExpire(testRedisKey, 30*time.Millisecond).Err()).NotTo(HaveOccurred())

		Expect(subject.Lock()).To(BeNil())

		Expect(redisClient.Get(testRedisKey).Result()).To(Equal(subject.token))
		Expect(subject.opts.TokenPrefix).To(Equal(subject.token[:len(subject.token)-24]))
		Expect(getTTL()).To(BeNumerically("~", time.Second, 10*time.Millisecond))

		Expect(subject.Unlock()).To(BeNil())
	})

	It("should not retry if not enabled", func() {
		Expect(redisClient.Set(testRedisKey, "ABCD", 0).Err()).NotTo(HaveOccurred())
		Expect(redisClient.PExpire(testRedisKey, 150*time.Millisecond).Err()).NotTo(HaveOccurred())
		subject.opts.RetryCount = 0

		Expect(subject.Lock()).ToNot(BeNil())
		Expect(getTTL()).To(BeNumerically("~", 150*time.Millisecond, 10*time.Millisecond))
	})

	It("should give up when retry count reached", func() {
		Expect(redisClient.Set(testRedisKey, "ABCD", 0).Err()).NotTo(HaveOccurred())
		Expect(redisClient.PExpire(testRedisKey, 150*time.Millisecond).Err()).NotTo(HaveOccurred())

		Expect(subject.Lock()).ToNot(BeNil())
		Expect(subject.token).To(Equal(""))

		Expect(redisClient.Get(testRedisKey).Result()).To(Equal("ABCD"))
		Expect(getTTL()).To(BeNumerically("~", 45*time.Millisecond, 20*time.Millisecond))
	})

	It("should failure on release expired lock", func() {
		Expect(subject.Lock()).To(BeNil())

		time.Sleep(subject.opts.LockTimeout * 2)

		err := subject.Unlock()
		Expect(err).To(Equal(ErrLockUnlockFailed))
	})

	It("should error when lock time exceeded while running handler", func() {
		err := Run(redisClient, testRedisKey, &Options{LockTimeout: time.Millisecond}, func() {
			time.Sleep(time.Millisecond * 5)
		})

		Expect(err).To(Equal(ErrLockDurationExceeded))
	})

	It("should retry and wait for locks if requested", func() {
		var (
			wg  sync.WaitGroup
			res int32
		)

		wg.Add(1)
		go func() {
			defer wg.Done()
			defer GinkgoRecover()

			err := Run(redisClient, testRedisKey, &Options{RetryCount: 10, RetryDelay: 10 * time.Millisecond}, func() {
				atomic.AddInt32(&res, 1)
			})
			Expect(err).NotTo(HaveOccurred())
		}()

		err := Run(redisClient, testRedisKey, nil, func() {
			atomic.AddInt32(&res, 1)
			time.Sleep(20 * time.Millisecond)
		})
		wg.Wait()

		Expect(err).NotTo(HaveOccurred())
		Expect(res).To(Equal(int32(2)))
	})

	It("should give up retrying after timeout", func() {
		var (
			wg  sync.WaitGroup
			res int32
		)

		wg.Add(1)
		go func() {
			defer wg.Done()
			defer GinkgoRecover()

			err := Run(redisClient, testRedisKey, &Options{RetryCount: 1, RetryDelay: 10 * time.Millisecond}, func() {
				atomic.AddInt32(&res, 1)
			})
			Expect(err).To(Equal(ErrLockNotObtained))
		}()

		err := Run(redisClient, testRedisKey, nil, func() {
			atomic.AddInt32(&res, 1)
			time.Sleep(100 * time.Millisecond)
		})
		wg.Wait()

		Expect(err).NotTo(HaveOccurred())
		Expect(res).To(Equal(int32(1)))
	})
})

// --------------------------------------------------------------------

func TestSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	AfterEach(func() {
		Expect(redisClient.Del(testRedisKey).Err()).NotTo(HaveOccurred())
	})
	RunSpecs(t, "redis-lock")
}

var redisClient *redis.Client

var _ = BeforeSuite(func() {
	redisClient = redis.NewClient(&redis.Options{
		Network: "tcp",
		Addr:    "127.0.0.1:6379", DB: 9,
	})
	Expect(redisClient.Ping().Err()).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	Expect(redisClient.Close()).To(Succeed())
})
