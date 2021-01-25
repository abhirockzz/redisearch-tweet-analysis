package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/RediSearch/redisearch-go/redisearch"
	"github.com/gomodule/redigo/redis"
)

var pool *redis.Pool
var rsClient *redisearch.Client

var indexName string

const (
	indexNameEnvVar = "REDISEARCH_INDEX_NAME"
	redisHost       = "REDIS_HOST"
	redisPassword   = "REDIS_PASSWORD"
	port            = "80"

	queryParamQuery       = "q"
	queryParamFields      = "fields"
	queryParamOffsetLimit = "offset_limit"

	responseHeaderSearchHits = "Search-Hits"
	responseHeaderPageSize   = "Page-Size"
)

func init() {
	host := GetEnvOrFail(redisHost)
	password := GetEnvOrFail(redisPassword)
	indexName = GetEnvOrFail(indexNameEnvVar)

	pool = &redis.Pool{Dial: func() (redis.Conn, error) {
		return redis.Dial("tcp", host, redis.DialPassword(password), redis.DialUseTLS(true), redis.DialTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12}))
	}}

	rsClient = redisearch.NewClientFromPool(pool, indexName)
}

func main() {

	http.HandleFunc("/search", search)
	log.Println("starting search api server")
	server := http.Server{Addr: ":" + port, Handler: nil}

	exit := make(chan os.Signal)
	signal.Notify(exit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-exit
		if pool != nil {
			err := pool.Close()
			if err != nil {
				log.Println("failed to close redis connection pool", err)
			}
		}
		server.Shutdown(context.Background())
	}()

	log.Fatal(server.ListenAndServe())
}

func search(rw http.ResponseWriter, req *http.Request) {

	qParams, err := url.ParseQuery(req.URL.RawQuery)
	if err != nil {
		log.Println("invalid query params")
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}

	searchQuery := qParams.Get(queryParamQuery)

	query := redisearch.NewQuery(searchQuery)

	fields := qParams.Get(queryParamFields)
	if fields != "" {
		log.Println("fields to be returned", fields)
		toBeReturned := strings.Split(fields, ",")
		query = query.SetReturnFields(toBeReturned...)
	}

	offsetAndLimit := qParams.Get(queryParamOffsetLimit)
	if offsetAndLimit != "" {
		log.Println("offset_limit", offsetAndLimit)
		offsetAndLimitVals := strings.Split(offsetAndLimit, ",")

		offset, err := strconv.Atoi(offsetAndLimitVals[0])
		if err != nil {
			http.Error(rw, "invalid offset", http.StatusBadRequest)
		}
		limit, err := strconv.Atoi(offsetAndLimitVals[1])
		if err != nil {
			http.Error(rw, "invalid limit", http.StatusBadRequest)
		}
		query = query.Limit(offset, limit)
	}

	docs, total, err := rsClient.Search(query)

	if err != nil {
		status := http.StatusInternalServerError

		if strings.Contains(err.Error(), "Syntax error") {
			status = http.StatusBadRequest
		}
		log.Println("search failed")
		http.Error(rw, err.Error(), status)
		return
	}
	fmt.Printf("Found %v docs matching query %s\n", total, searchQuery)
	fmt.Printf("Showing %v docs in results as per offset and limit %v\n", len(docs), query.Paging)

	response := []map[string]interface{}{}
	for _, doc := range docs {
		response = append(response, doc.Properties)
	}

	rw.Header().Add(responseHeaderSearchHits, strconv.Itoa(total))
	rw.Header().Add(responseHeaderPageSize, strconv.Itoa(len(docs)))
	err = json.NewEncoder(rw).Encode(response)
	if err != nil {
		log.Println("failed to encode response")
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
}

func GetEnvOrFail(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("Environment variable %s not set", key)
	}

	return val
}
