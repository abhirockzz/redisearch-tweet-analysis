package twitter

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/abhirockzz/redisearch-go-app/index"
	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"
)

const (
	consumerKeyEnvVar       = "TWITTER_CONSUMER_KEY"
	consumerSecretKeyEnvVar = "TWITTER_CONSUMER_SECRET_KEY"
	accessTokenEnvVar       = "TWITTER_ACCESS_TOKEN"
	accessSecretEnvVar      = "TWITTER_ACCESS_SECRET_TOKEN"
)

// StartStream starts listening to tweet stream
func StartStream() *twitter.Stream {

	config := oauth1.NewConfig(GetEnvOrFail(consumerKeyEnvVar), GetEnvOrFail(consumerSecretKeyEnvVar))
	token := oauth1.NewToken(GetEnvOrFail(accessTokenEnvVar), GetEnvOrFail(accessSecretEnvVar))
	httpClient := config.Client(oauth1.NoContext, token)

	// Twitter client
	client := twitter.NewClient(httpClient)

	params := &twitter.StreamSampleParams{
		Language:      []string{"en"},
		StallWarnings: twitter.Bool(true),
	}

	var err error
	stream, err := client.Streams.Sample(params)

	if err != nil {
		log.Fatalf("error %v", err)
	}

	fmt.Println("connected to twitter sample stream")

	demux := twitter.NewSwitchDemux()
	demux.Tweet = func(tweet *twitter.Tweet) {

		if !tweet.PossiblySensitive {
			log.Printf("processing tweet from %s %s %s source: %s loc: %s\n", tweet.User.ScreenName, tweet.IDStr, tweet.Text, tweet.Source, tweet.User.Location)

			//index.AddData(tweet.IDStr, tweet.User.ScreenName, tweet.Text, source, allHashtags)
			go index.AddData(tweetToMap(tweet))
			time.Sleep(3 * time.Second) //on purpose
		}
	}

	go func() {
		fmt.Println("twitter stream started>>>>")

		for tweet := range stream.Messages {
			demux.Handle(tweet)
		}
	}()

	return stream

}

func tweetToMap(tweet *twitter.Tweet) map[string]interface{} {
	tweetData := make(map[string]interface{})

	tweetData["id"] = tweet.IDStr
	tweetData["user"] = tweet.User.ScreenName
	tweetData["text"] = tweet.Text

	hashtags := []string{}
	for _, hash := range tweet.Entities.Hashtags {
		hashtags = append(hashtags, hash.Text)
	}
	allHashtags := strings.Join(hashtags, ",")
	if allHashtags != "" {
		log.Println("Hashtags", allHashtags)
		tweetData["hashtags"] = allHashtags
	}

	source := "unknown"
	if tweet.Source != "" {
		sub := `"nofollow">`
		source = tweet.Source[strings.Index(tweet.Source, sub)+len(sub) : strings.Index(tweet.Source, "</a>")]
	}
	tweetData["source"] = source
	fmt.Println("source", source)

	loc := tweet.User.Location
	if loc != "" {
		tweetData["location"] = loc
		fmt.Println("location", loc)
	}

	t, err := tweet.CreatedAtTime()
	if err == nil {
		fmt.Println("tweet was created on", t.String())
		created := t.UTC().UnixNano()
		tweetData["created"] = created
		fmt.Println("creation timestamp", created)
	}

	coords := tweet.Coordinates
	if coords != nil {
		long := fmt.Sprintf("%f", coords.Coordinates[0])
		lat := fmt.Sprintf("%f", coords.Coordinates[1])

		coordinates := fmt.Sprintf("%s %s", long, lat)
		fmt.Println("coordinates", coordinates)
		tweetData["coordinates"] = coordinates
	}

	return tweetData
}

// GetEnvOrFail gets value for given env var or exits with an error message if env var is not present
func GetEnvOrFail(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("Environment variable %s not set", key)
	}

	return val
}
