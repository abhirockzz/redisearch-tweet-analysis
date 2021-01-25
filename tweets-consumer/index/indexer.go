package index

import (
	"crypto/tls"
	"fmt"
	"log"
	"os"

	"github.com/RediSearch/redisearch-go/redisearch"
	"github.com/gomodule/redigo/redis"
)

var pool *redis.Pool

const (
	indexNameEnvVar           = "REDISEARCH_INDEX_NAME"
	redisHost                 = "REDIS_HOST"
	redisPassword             = "REDIS_PASSWORD"
	indexDefinitionHashPrefix = "tweet:"
)

var indexName string

func init() {

	host := GetEnvOrFail(redisHost)
	password := GetEnvOrFail(redisPassword)
	indexName = GetEnvOrFail(indexNameEnvVar)

	pool = &redis.Pool{Dial: func() (redis.Conn, error) {
		return redis.Dial("tcp", host, redis.DialPassword(password), redis.DialUseTLS(true), redis.DialTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12}))
	}}

	dropAndCreateIndex()
}

// drops the index first (if present) along with ALL the documents associated with it and then re-creates the index
func dropAndCreateIndex() {
	rsClient := redisearch.NewClientFromPool(pool, indexName)
	err := rsClient.DropIndex(true)
	if err != nil {
		log.Println("drop index failed ", err)
	} else {
		log.Println("index dropped")
	}

	schema := redisearch.NewSchema(redisearch.DefaultOptions).
		AddField(redisearch.NewTextFieldOptions("id", redisearch.TextFieldOptions{})).
		AddField(redisearch.NewTextFieldOptions("user", redisearch.TextFieldOptions{})).
		AddField(redisearch.NewTextFieldOptions("text", redisearch.TextFieldOptions{})).
		AddField(redisearch.NewTextFieldOptions("source", redisearch.TextFieldOptions{})).
		//tags are comma-separated by default
		AddField(redisearch.NewTagFieldOptions("hashtags", redisearch.TagFieldOptions{})).
		AddField(redisearch.NewTextFieldOptions("location", redisearch.TextFieldOptions{})).
		AddField(redisearch.NewNumericFieldOptions("created", redisearch.NumericFieldOptions{Sortable: true})).
		AddField(redisearch.NewGeoFieldOptions("coordinates", redisearch.GeoFieldOptions{}))

	indexDefinition := redisearch.NewIndexDefinition().AddPrefix(indexDefinitionHashPrefix)

	err = rsClient.CreateIndexWithIndexDefinition(schema, indexDefinition)
	if err != nil {
		log.Fatal("index creation failed ", err)
	}

	log.Println("index created")
}

// AddData adds tweet info to a HASH
func AddData(tweetData map[string]interface{}) {

	conn := pool.Get()

	hashName := fmt.Sprintf("tweet:%s", tweetData["id"])
	val := redis.Args{hashName}.AddFlat(tweetData)

	_, err := conn.Do("HSET", val...)
	if err != nil {
		log.Println("failed to add tweet info", err)
		return
	}

	log.Println("added tweet info to redis: ", tweetData["id"])
}

// Close closes redis connection pool
func Close() {
	if pool != nil {
		err := pool.Close()
		if err != nil {
			log.Println("failed to close redis connection pool", err)
		}
	}
}

// GetEnvOrFail fetches value for the given env var or fails with a message
func GetEnvOrFail(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("Environment variable %s not set", key)
	}

	return val
}
