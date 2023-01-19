package main

import (
	"context"
	"fmt"
	"log"

	"github.com/iAlexBLR/stardog-sdk-go/stardog"
)

func main() {
	client := stardog.NewClient(nil, "http://127.0.0.1:5820/")
	client.SetBasicAuth("admin", "admin")

	users, _, err := client.Users.List(context.Background())
	if err != nil {
		log.Fatalf("Error listing users: %v", err)
	}

	for _, value := range users.Users {
		fmt.Println(value)
	}
}
