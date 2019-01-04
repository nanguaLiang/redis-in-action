package ch1

import (
	"fmt"
	"github.com/gomodule/redigo/redis"
	"testing"
)

/*
First run,the articleId is 1 in empty redis database
*/
func TestPostArticle(t *testing.T) {
	conn, _ := redis.Dial("tcp", "localhost:6379")
	defer conn.Close()
	user := "user:1"
	title := "This is a title"
	link := "127.0.0.1:6379"
	articleID, err := PostArticle(conn, user, title, link)
	if err != nil {
		t.Error(err)
	}

	article, _ := redis.StringMap(conn.Do("HGETALL", fmt.Sprintf("article:%s", articleID)))
	if article["poster"] != user && article["title"] != title && article["link"] != link {
		t.Errorf("expected %s %s %s got %s %s %s", user, title, link, article["poster"], article["title"], article["link"])
	}
}

func TestArticleVote(t *testing.T) {
	conn, _ := redis.Dial("tcp", "localhost:6379")
	defer conn.Close()
	article := "article:2"
	user := "user:3"
	oldVote, err := redis.Int(conn.Do("HGET", article, "votes"))
	if err != nil {
		t.Error(err)
	}

	err = ArticleVote(conn, user, article)
	if fmt.Sprintf("%v", err) == "This user has already voted" {
		t.Error(err)
	}

	voteNum, err := redis.Int(conn.Do("HGET", article, "votes"))
	if err != nil {
		t.Error(err)
	}

	if voteNum != oldVote+1 {
		t.Errorf("expected %v got %v", oldVote+1, voteNum)
	}
}

func TestGetArticles(t *testing.T) {
	conn, _ := redis.Dial("tcp", "localhost:6379")
	defer conn.Close()
	articles, err := GetArticles(conn, 1, "score:")
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 1 {
		t.Errorf("expected %d got %d", 1, len(articles))
	}
	if articles[0]["user"] != "user:1" && articles[0]["title"] != "111" && articles[0]["link"] != "222" {
		t.Errorf("expected %s %s %s got %s %s %s", "user:1", "This is a title", "127.0.0.1:6379", articles[0]["user"], articles[0]["title"], articles[0]["link"])
	}
}

//redis  the key(set) group:programming article:1
func TestAddAndRemoveToGroup(t *testing.T) {
	conn, _ := redis.Dial("tcp", "localhost:6379")
	defer conn.Close()
	AddAndRemoveToGroup(conn, int64(1), []string{"programming"}, []string{})
	articleId, err := redis.Strings(conn.Do("SMEMBERS", "group:programming"))
	if err != nil {
		t.Fatal(err)
	}
	if articleId[0] != "article:1" {
		t.Errorf("expected article:1 got %s", articleId)
	}
}

//redis the key(zset) score:programming article:1
func TestGetGroupArticles(t *testing.T) {
	conn, _ := redis.Dial("tcp", "localhost:6379")
	defer conn.Close()
	_, err := GetGroupArticles(conn, 1, "programming", "score:")
	if err != nil {
		t.Fatal(err)
	}
	articleId, err := redis.Strings(conn.Do("ZRANGE", "score:programming", 0, -1))
	if err != nil {
		t.Fatal(err)
	}
	if articleId[0] != "article:1" {
		t.Errorf("expected article:1 got %s", articleId)
	}
}
