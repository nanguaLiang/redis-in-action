package ch2

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/gomodule/redigo/redis"
	"github.com/pkg/errors"
	"hash/fnv"
	"log"
	"math"
	"net/url"
	"strconv"
	"time"
)

//2.1
func CheckToken(conn redis.Conn, token string) (string, error) {
	userId, err := redis.String(conn.Do("HGET", "login:", token))
	if err != nil {
		return "", err
	}
	return userId, nil
}

//2.2 && 2.9
func UpdateToken(conn redis.Conn, token string, user string, item string) error {
	timestamp := time.Now().Unix()
	_, err := conn.Do("HSET", "login:", token, user)
	if err != nil {
		return err
	}
	_, err = conn.Do("ZADD", "recent:", timestamp, token)
	if err != nil {
		return err
	}
	if item != "" {
		_, err := conn.Do("ZADD", fmt.Sprintf("viewed:%s", token), timestamp, item)
		if err != nil {
			return err
		}
		_, err = conn.Do("ZREMRANGEBYRANK", fmt.Sprintf("viewed:%s", token), 0, -26)
		if err != nil {
			return err
		}
		_, err = conn.Do("ZINCRBY", "viewed:", 1, item)
		if err != nil {
			return err
		}

	}
	return nil
}

//2.3&&2.5 daemon
func CleanSessions(conn redis.Conn) {
	var (
		quit  = false
		limit = 10000000
	)
	for {
		if quit {
			break
		}
		size, err := redis.Int(conn.Do("ZCARD", "recent:"))
		if err != nil {
			log.Println(errors.New("get token num error"))
		}
		if size < limit {
			time.Sleep(time.Second)
			continue
		}
		endIndex := int(math.Min(float64(size-limit), 100))
		tokens, err := redis.Strings(conn.Do("ZRANGE", "recent:", 0, endIndex))
		if err != nil {
			log.Println(errors.New("get tokens error"))
		}
		sessionKeys := make([]string, 0)
		for _, token := range tokens {
			sessionKeys = append(sessionKeys, fmt.Sprintf("viewed:%s", token))
			sessionKeys = append(sessionKeys, fmt.Sprintf("cart:%s", token))
		}
		//bulk delete
		conn.Do("MULTI")
		for _, key := range sessionKeys {
			conn.Do("DEL", key)
		}
		conn.Do("EXEC")

		conn.Do("MULTI")
		for _, token := range tokens {
			conn.Do("HDEL", fmt.Sprintf("login:%s", token))
		}
		conn.Do("EXEC")

		conn.Do("MULTI")
		for _, token := range tokens {
			conn.Do("ZREM", fmt.Sprintf("recent:%s", token))
		}
		conn.Do("EXEC")
	}
}

//2.4
func AddToCart(conn redis.Conn, session string, item string, count int) error {
	if count <= 0 {
		_, err := conn.Do("HREM", fmt.Sprintf("cart:%s", session), item)
		if err != nil {
			return err
		}
	}
	_, err := conn.Do("HSET", fmt.Sprintf("cart:%s", session), item, count)
	if err != nil {
		return err
	}
	return nil
}

//2.6
type Callback interface {
	Callback(string) string
}

func CacheRequest(conn redis.Conn, request string, callback Callback) string {
	if !CanCache(conn, request) {
		return callback.Callback(request)
	}
	pageKey := fmt.Sprintf("cache:%s", hashRequest(request))
	content, _ := redis.String(conn.Do("GET", pageKey))
	if content == "" {
		content = callback.Callback(request)
		conn.Do("SET", pageKey, content, "EX", 300)
	}
	return content
}

func CanCache(conn redis.Conn, request string) bool {
	parsed, _ := url.Parse(request)
	params := parsed.Query()
	itemId := extractItemId(params)
	if itemId == "" || isDynamic(params) {
		return false
	}
	rank, err := redis.Int64(conn.Do("ZRANK", "viewed:", itemId))
	if err == nil && rank < 10000 {
		return true
	}
	return false
}

func isDynamic(params url.Values) bool {
	if _, ok := params["_"]; ok {
		return true
	}
	return false
}

func extractItemId(params url.Values) string {
	return params.Get("item")
}

func hashRequest(request string) string {
	h := fnv.New32a()
	h.Write([]byte(request))
	return hex.EncodeToString(h.Sum(nil))
}

//2.7
func ScheduleRowCache(conn redis.Conn, rowId string, delay time.Duration) error {
	_, err := conn.Do("ZADD", "delay:", delay, rowId)
	if err != nil {
		return err
	}
	_, err = conn.Do("ZADD", "schedule:", time.Now().Unix(), rowId)
	if err != nil {
		return err
	}
	return nil
}

//2.8 daemon
func CacheRows(conn redis.Conn) {
	var quit = false
	for {
		if quit {
			break
		}
		next, err := redis.Strings(conn.Do("ZRANGE", "schedule:", 0, 0, "WITHSCORES"))
		now := time.Now().Unix()
		if err != nil {
			log.Println(err)
		}
		if len(next) != 0 || next[1] > strconv.FormatInt(now, 10) {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		rowId := next[0]
		delay, err := redis.Int64(conn.Do("ZSCORE", "delay:", rowId))
		if err != nil {
			log.Println(err)
		}
		if delay <= 0 {
			conn.Do("ZREM", "delay:", rowId)
			conn.Do("ZREM", "schedule:", rowId)
			conn.Do("DELETE", fmt.Sprintf("inv:%s", rowId))
			continue
		}
		row := get(rowId)
		conn.Do("ZADD", "schedule:", now+delay, rowId)
		record, err := json.Marshal(row)
		if err != nil {
			panic(err)
		}
		conn.Do("SET", fmt.Sprintf("inv:%s", rowId), record)

	}
}

type Inventory struct {
	RowId string
	Data  string
	Time  string
}

//mock
func get(rowId string) Inventory {
	in := Inventory{
		RowId: rowId,
		Data:  "data to cache...",
		Time:  strconv.FormatInt(time.Now().Unix(), 10),
	}
	return in
}

//2.10 daemon
func RescaleViewed(conn redis.Conn) {
	var (
		quit = false
	)
	for {
		if quit {
			break
		}
		_, err := conn.Do("ZREMRANGEBYRANK", "viewed:", 0, -20001)
		if err != nil {
			log.Println(err)
		}
		_, err = conn.Do("ZINTERSTORE", "viewed:", 1, "viewed:", "WEIGHTS", 0.5)
		if err != nil {
			log.Println(err)
		}
		time.Sleep(500 * time.Second)

	}
}
