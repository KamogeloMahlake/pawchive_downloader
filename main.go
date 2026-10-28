package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/KamogeloMahlake/pawchive_downloader/core/clients"
	"github.com/KamogeloMahlake/pawchive_downloader/pawchive"
)

const baseUrl = "https://pawchive.pw/api/v1"

func main() {
	client := clients.Client{
		Client: &http.Client{
			Timeout: 5 * time.Second,
		},	
	}
/*
	var posts []pawchive.Post
	err := client.GetPosts(baseUrl, "fanbox", 37736420, &posts)
	*/
	var creator pawchive.Creator
	err := client.GetCreator(baseUrl, "fanbox", 37736420, &creator)
	if err != nil {
		log.Fatalf("%v", err)
	}
	fmt.Printf("%+v\n", creator)
}
