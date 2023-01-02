package main

import (
	"context"
	"fmt"

	"github.com/iAlexBLR/stardog-sdk-go/stardog"
)

func main() {
	client := stardog.NewClient(nil, "http://127.0.0.1:5820/")
	client.SetBasicAuth("admin", "admin")

	_, err := client.Users.List(context.Background())
	if err != nil {
		fmt.Println(err)
	}
}
