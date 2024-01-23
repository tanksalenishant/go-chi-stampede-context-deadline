package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/stampede"
	"github.com/streadway/amqp"
)

var timeA = 5 * time.Second
var timeB = 2 * time.Second
var timeLimit = 3 * time.Second

var rabbitMQURL = "amqp://guest:guest@localhost:5672/" //username : guest password : guest

func requestA(ctx context.Context, resultChan chan<- error) {
	select {
	case <-time.After(timeA):
		fmt.Println("Request A completed successfully.")
		resultChan <- nil
	case <-ctx.Done():
		resultChan <- fmt.Errorf("Request A canceled: Time Limit Exceeded: %v", ctx.Err())
	}
}

func requestB(ctx context.Context, resultChan chan<- error) {
	select {
	case <-time.After(timeB):
		fmt.Println("Request B completed successfully.")
		resultChan <- nil
	case <-ctx.Done():
		resultChan <- fmt.Errorf("Request B canceled: Time Limit Exceeded: %v", ctx.Err())
	}
}

func enqueueRequest(requestType string) error {
	conn, err := amqp.Dial(rabbitMQURL)
	if err != nil {
		return fmt.Errorf("Failed to connect to RabbitMQ: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("Failed to open a channel: %v", err)
	}
	defer ch.Close()

	queueName := "requests_queue"
	_, err = ch.QueueDeclare(queueName, true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("Failed to declare a queue: %v", err)
	}

	requestData := map[string]string{
		"type": requestType,
		"time": time.Now().Format(time.RFC3339),
	}

	requestDataStr := fmt.Sprintf("%v", requestData)

	err = ch.Publish("", queueName, false, false, amqp.Publishing{
		ContentType: "text/plain",
		Body:        []byte(requestDataStr),
	})
	if err != nil {
		return fmt.Errorf("Failed to publish a message: %v", err)
	}

	return nil
}

func processEnqueuedRequests() {
	conn, err := amqp.Dial(rabbitMQURL)
	if err != nil {
		fmt.Println("Failed to connect to RabbitMQ:", err)
		return
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		fmt.Println("Failed to open a channel:", err)
		return
	}
	defer ch.Close()

	queueName := "requests_queue"
	msgs, err := ch.Consume(queueName, "", true, false, false, false, nil)
	if err != nil {
		fmt.Println("Failed to consume messages from the queue:", err)
		return
	}

	for msg := range msgs {
		go processEnqueuedRequest(string(msg.Body))
	}
}

func processEnqueuedRequest(requestDataStr string) {
	time.Sleep(10 * time.Second) 
	fmt.Println("Received enqueued request:", requestDataStr)
	fmt.Println("Processed enqueued request." , requestDataStr)
}

func enqueueAndProcess(requestType string) {
	time.Sleep(10 * time.Second) 

	if err := enqueueRequest(requestType); err != nil {
		fmt.Printf("Error enqueuing request %s: %v\n", requestType, err)
	}
}

func requestAHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeLimit)
	defer cancel()

	resultChan := make(chan error)

	go requestA(ctx, resultChan)

	select {
	case err := <-resultChan:
		if err != nil {
			go enqueueAndProcess("A")
			http.Error(w, fmt.Sprintf("Error from Request A: %v", err), http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, "Request A completed within the timeout.")
	}
}

func requestBHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeLimit)
	defer cancel()

	resultChan := make(chan error)

	go requestB(ctx, resultChan)

	select {
	case err := <-resultChan:
		if err != nil {
			go enqueueAndProcess("B")
			http.Error(w, fmt.Sprintf("Error from Request B: %v", err), http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, "Request B completed within the timeout.")
	}
}

func main() {
	go processEnqueuedRequests()

	r := chi.NewRouter()
	cached := stampede.Handler(512, 3*time.Second)
	r.With(cached).Get("/requestA", requestAHandler)
	r.With(cached).Get("/requestB", requestBHandler)

	http.ListenAndServe(":8080", r)
}
