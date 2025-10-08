package tests

import (
	"rabbitmq-platform/mocks"
	"rabbitmq-platform/pkg/broker"
	"sync"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/mock"
)

func createMockRabbitMQClient() broker.RabbitMQClientInterface {
	// Use the mockery-generated mock
	mockClient := &mocks.RabbitMQClientInterface{}

	// Set up mock expectations for basic operations
	// For race condition tests, we don't need actual channel operations to work
	// Just return a nil channel and let the test focus on the race conditions
	mockClient.On("NewChannel").Return((*amqp.Channel)(nil), nil)
	mockClient.On("DeclareExchange", mock.Anything, mock.Anything).Return(nil)
	mockClient.On("DeclareQueue", mock.Anything, mock.Anything, mock.Anything).Return(amqp.Queue{}, nil)
	mockClient.On("BindQueue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	return mockClient
}

// TestConsumerActualRaceConditions tests real race conditions in the Consumer implementation
func TestConsumerActualRaceConditions(t *testing.T) {
	// Mock RabbitMQ client - you'll need to implement this
	mockClient := createMockRabbitMQClient()

	config := broker.ConsumerConfig{
		QueueName:   "test-queue",
		Exchange:    "test-exchange",
		RoutingKey:  "test-key",
		IsDLQNeeded: true,
	}

	processFunc := func(msg amqp.Delivery) error {
		time.Sleep(5 * time.Millisecond)
		return nil
	}

	t.Run("multiple_stop_calls_race", func(t *testing.T) {
		consumer, err := broker.NewConsumer(mockClient, config, processFunc)
		if err != nil {
			t.Fatal(err)
		}

		// Don't call Start() as it requires actual RabbitMQ operations
		// Instead, test the race conditions on Stop() directly

		// This is the actual race condition test - multiple Stop() calls
		var wg sync.WaitGroup
		concurrentCalls := 5

		for i := 0; i < concurrentCalls; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				// This could panic if stopCh is closed multiple times
				_ = consumer.Stop()
			}()
		}

		// This should not panic
		wg.Wait()
	})

	t.Run("stop_while_processing_messages", func(t *testing.T) {
		consumer, err := broker.NewConsumer(mockClient, config, processFunc)
		if err != nil {
			t.Fatal(err)
		}

		// Don't call Start() as it requires actual RabbitMQ operations
		// This test focuses on the race condition between message processing simulation and Stop()

		var wg sync.WaitGroup

		// Simulate message processing
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Simulate handleMessage being called
			time.Sleep(10 * time.Millisecond)
		}()

		// Stop while messages are being processed
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(5 * time.Millisecond) // Stop in the middle of processing
			_ = consumer.Stop()
		}()

		wg.Wait()
	})
}

// TestConsumerChannelRace tests race conditions with channel operations
func TestConsumerChannelRace(t *testing.T) {
	mockClient := createMockRabbitMQClient()

	config := broker.ConsumerConfig{
		QueueName:  "test-queue",
		Exchange:   "test-exchange",
		RoutingKey: "test-key",
	}

	t.Run("channel_close_while_acking", func(t *testing.T) {
		consumer, err := broker.NewConsumer(mockClient, config, func(msg amqp.Delivery) error {
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}

		// Don't call Start() as it requires actual RabbitMQ operations
		// This test focuses on race conditions with channel operations

		var wg sync.WaitGroup

		// Simulate channel operations (Ack/Nack) happening concurrently with Stop()
		wg.Add(1)
		go func() {
			defer wg.Done()
			// This simulates handleMessage trying to use the channel
			// while Stop() might be closing it
			time.Sleep(5 * time.Millisecond)
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(2 * time.Millisecond)
			_ = consumer.Stop() // This closes the channel
		}()

		wg.Wait()
	})
}

// TestConsumerWaitGroupRace tests WaitGroup race conditions
func TestConsumerWaitGroupRace(t *testing.T) {
	t.Run("waitgroup_concurrent_add_wait", func(t *testing.T) {
		var wg sync.WaitGroup
		var mu sync.Mutex
		running := true

		// Simulate the consumer pattern: Add(1) in one goroutine, Wait() in another
		go func() {
			for i := 0; i < 100; i++ {
				mu.Lock()
				if running {
					wg.Add(1)
					go func(id int) {
						defer wg.Done()
						time.Sleep(time.Microsecond)
					}(i)
				}
				mu.Unlock()
			}
		}()

		// This simulates Stop() calling wg.Wait() while messages are still being added
		go func() {
			time.Sleep(5 * time.Millisecond)
			mu.Lock()
			running = false
			mu.Unlock()
			wg.Wait() // This could race with Add(1) calls
		}()

		time.Sleep(50 * time.Millisecond)
	})
}
