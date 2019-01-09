package ch2

import (
	"fmt"
	"github.com/gomodule/redigo/redis"
	"github.com/satori/go.uuid"
	"testing"
)

func TestLoginCookies(t *testing.T) {
	conn, err := redis.Dial("tcp", "localhost:6379")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	token := uuid.Must(uuid.NewV4()).String()
	err = UpdateToken(conn, token, "user1", "item1")
	if err != nil {
		t.Fatal(err)
	}
	user, err := CheckToken(conn, token)
	if err != nil {
		t.Fatal(err)
	}
	if user != "user1" {
		t.Errorf("expected user1 got %s", user)
	}
}

func TestAddToCart(t *testing.T) {
	conn, err := redis.Dial("tcp", "localhost:6379")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	token := uuid.Must(uuid.NewV4()).String()
	err = UpdateToken(conn, token, "user2", "item2")
	if err != nil {
		t.Fatal(err)
	}
	AddToCart(conn, token, "item3", 3)
	things, err := redis.StringMap(conn.Do("HGETALL", fmt.Sprintf("cart:%s", token)))
	if err != nil {
		t.Fatal(err)
	}
	if things["item3"] != "3" {
		t.Errorf("expected 3 got %s", things["item3"])
	}

}

type callback struct {
}

func (*callback) Callback(request string) string {
	return "content for " + request
}

func TestCacheRequest(t *testing.T) {
	conn, err := redis.Dial("tcp", "localhost:6379")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	token := uuid.Must(uuid.NewV4()).String()
	err = UpdateToken(conn, token, "user4", "item4")
	if err != nil {
		t.Fatal(err)
	}

	request := "http://test.com?item=item4"
	isCanCache := CanCache(conn, request)
	if !isCanCache {
		t.Errorf("expected true got %t", isCanCache)
	}

	requestNot := "http://test.com"
	isCanCache = CanCache(conn, requestNot)
	if isCanCache {
		t.Errorf("expected false got %t", isCanCache)
	}

	content := CacheRequest(conn, request, &callback{})
	if content == "" {
		t.Fatal("the cache doesn't work")
	}
	if content != fmt.Sprintf("content for %s", request) {
		t.Errorf("expected %s got %s", fmt.Sprintf("content for %s", request), content)
	}

}
