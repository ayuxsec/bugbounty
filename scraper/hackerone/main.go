package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

var (
	programHandlesPath   = flag.String("input-path", "", "path to program handles file to scrape")
	outputPath           = flag.String("output-path", "", "jsonl file to write scraped data")
	maxRequestsPerMinute = flag.Int("rlm", 600, "max requests to send per minute. see https://api.hackerone.com/getting-started/#rate-limits")
	apiCreds             = flag.String("api", "", "usernmae:key format api key to use. see https://hackerone.com/settings/api_token/edit")
)

func main() {
	flag.Parse()
	if *apiCreds == "" || *programHandlesPath == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	limiter := rate.NewLimiter(
		rate.Every(time.Minute/time.Duration(*maxRequestsPerMinute)),
		*maxRequestsPerMinute,
	)

	file := Must(os.Open(*programHandlesPath))
	defer file.Close()
	scanner := bufio.NewScanner(file)

	client := http.Client{}

	for scanner.Scan() {
		pHandle := scanner.Text()
		if err := limiter.Wait(context.Background()); err != nil {
			log.Fatalf("rate.Limiter.Wait error: %+v", err)
		}
		req := Must(
			http.NewRequest(
				"GET",
				fmt.Sprintf("https://api.hackerone.com/v1/hackers/programs/%s/structured_scopes", pHandle),
				nil,
			),
		)
		req.SetBasicAuth(strings.Split(*apiCreds, ":")[0], strings.Split(*apiCreds, ":")[1])
		resp, err := client.Do(req)
		if err != nil {
			log.Fatalf("client.Do error: +%v", err)
		}
		defer resp.Body.Close()

		data := Must(io.ReadAll(resp.Body))

		if *outputPath != "" {
			f := Must(os.OpenFile(
				*outputPath,
				os.O_CREATE|os.O_WRONLY|os.O_APPEND,
				0644,
			))
			defer f.Close()

			_, err := f.Write(append(data, '\n')) // newline for jsonl
			if err != nil {
				log.Fatalf("write error: %v", err)
			}
		} else {
			log.Print(string(data))
		}

		log.Print("[+] success scraping program: ", pHandle)
	}
	if scanner.Err() != nil {
		log.Fatal(scanner.Err().Error())
	}
}

func Must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
