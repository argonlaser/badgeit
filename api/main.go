package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/garyburd/redigo/redis"

	"github.com/gin-gonic/gin"
)

const (
	downloadTypeGit  string = "git"
	downloadTypeCurl string = "curl"
)

func main() {
	log.Println("Booting Badgeit API server")

	log.Println("Initializing Redis Message Queue")
	conn, err := redis.Dial("tcp", fmt.Sprintf("%s:%s", os.Getenv("REDIS_HOSTNAME"), os.Getenv("REDIS_PORT")))
	if err != nil {
		log.Fatalln("Failed to initialize redis message queue", err)
	}
	defer conn.Close()
	log.Println("Initialized Redis Message Queue successfully")

	log.Println("Initializing API Server")
	initAPIServer(conn)
}

func initAPIServer(redisConn redis.Conn) {
	r := gin.Default()

	r.POST("/test/callback", func(c *gin.Context) {
		io.Copy(os.Stdout, c.Request.Body)
		defer c.Request.Body.Close()
		c.JSON(http.StatusOK, gin.H{
			"test": "ok",
		})
		return
	})

	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"app": "badgeit-api",
			"version": "experimental",
		})
		return
	})

	r.GET("/badges", func(c *gin.Context) {
		downloadType, hasDownloadType := c.GetQuery("download")
		if !hasDownloadType || downloadType == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "download is a required",
			})
			return
		}
		if downloadType != downloadTypeGit && downloadType != downloadTypeCurl {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("Allowed download types are %s, %s", downloadTypeGit, downloadTypeCurl),
			})
			return
		}

		remote, hasRemote := c.GetQuery("remote")
		if !hasRemote || remote == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "remote is a required",
			})
			return
		}

		callback, hasCallback := c.GetQuery("callback")
		if !hasCallback || callback == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "callback is a required",
			})
			return
		}

		// check for cached data
		cachedData, _ := redis.String(redisConn.Do("GET", fmt.Sprintf("badge:%s", remote)))

		payload := gin.H{
			"download": downloadType,
			"remote":   remote,
			"callback": callback,
			"cache":    cachedData,
		}

		// check if worker is already working on badge computation
		alreadyProcessing, err := redis.Bool(redisConn.Do("SISMEMBER", "badgeit:processingRemotes", remote))
		if err != nil {
			log.Println("Unable to check if remote is already processing due to err", err)
		}
		if alreadyProcessing {
			payload["status"] = "already processing"
			c.JSON(http.StatusOK, payload)
			return
		}

		alreadyQueued, err := redis.Bool(redisConn.Do("SISMEMBER", "badgeit:queuedRemotes", remote))
		if err != nil {
			log.Println("Unable to check if remote is already queued due to err", err)
		}
		if alreadyQueued {
			payload["status"] = "already queued for processing"
			c.JSON(http.StatusOK, payload)
			return
		}

		// queue a task for the worker
		jsonPayload, _ := json.Marshal(payload)

		redisConn.Send("MULTI")
		redisConn.Send("SADD", "badgeit:queuedRemotes", remote)
		redisConn.Send("LPUSH", "badge:worker", []byte(jsonPayload))
		_, err = redisConn.Do("EXEC")
		if err != nil {
			log.Println("Unable to queue request", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "Unable to queue request",
			})
			return
		}

		payload["status"] = "successfully queued for processing"
		c.JSON(http.StatusAccepted, payload)
		return
	})
	r.Run(":8080")
}
