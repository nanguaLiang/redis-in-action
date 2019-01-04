package ch1

import (
	"fmt"
	"github.com/gomodule/redigo/redis"
	"github.com/pkg/errors"
	"strconv"
	"strings"
	"time"
)

const (
	ONE_WEEK_IN_SECONDS = 7 * 86400
	VOTE_SCORE          = 432
	ARTICLE_PER_PAGE    = 25
)

/*
1-3
*/
func ArticleVote(conn redis.Conn, user, article string) error {
	cutoff := time.Now().Unix() - ONE_WEEK_IN_SECONDS
	articleTime, err := redis.Int64(conn.Do("ZSCORE", "time:", article))
	if err != nil {
		return err
	}
	if articleTime < cutoff {
		return errors.New("Voting time has expired")
	}
	//time.Now().Unix()-articleTime<ONE_WEEK_IN_SECONDS
	articleID := strings.Split(article, ":")[1]
	if res, _ := redis.Int(conn.Do("SADD", fmt.Sprintf("voted:%s", articleID), user)); res != 0 {
		conn.Do("ZINCRBY", "score:", VOTE_SCORE, article)
		conn.Do("HINCRBY", article, "votes", 1)
		return nil
	} else {
		return errors.New("This user has already voted")
	}
}

func PostArticle(conn redis.Conn, user, title, link string) (string, error) {
	articleID, err := redis.Int(conn.Do("INCR", "article:"))
	if err != nil {
		return "", err
	}
	voted := "voted:" + strconv.Itoa(articleID)
	conn.Do("SADD", voted, user)
	conn.Do("EXPIRE", voted, ONE_WEEK_IN_SECONDS)
	postTime := time.Now().Unix()
	article := "article:" + strconv.Itoa(articleID)
	ok, err := redis.String(conn.Do("HMSET", article, "title", title, "link", link, "poster", user, "time", postTime, "votes", 1))
	if err != nil && ok != "OK" {
		return "", errors.New("post article fail")
	}

	conn.Do("ZADD", "score:", postTime+VOTE_SCORE, article)
	conn.Do("ZADD", "time:", postTime, article)

	return strconv.Itoa(articleID), nil
}

func GetArticles(conn redis.Conn, page int, order string) ([]map[string]string, error) {
	start := (page - 1) * ARTICLE_PER_PAGE
	end := start + ARTICLE_PER_PAGE - 1

	articleIDs, err := redis.Strings(conn.Do("ZREVRANGE", order, start, end))
	if err != nil {
		return nil, err
	}

	var articles []map[string]string

	for _, id := range articleIDs {
		articlesData, err := redis.StringMap(conn.Do("HGETALL", id))
		if err != nil {
			return nil, err
		}
		articlesData["id"] = id
		articles = append(articles, articlesData)
	}
	return articles, nil
}

func AddAndRemoveToGroup(conn redis.Conn, articleId int64, add, remove []string) {
	article := "article:" + strconv.FormatInt(articleId, 10)
	for _, group := range add {
		conn.Do("SADD", fmt.Sprintf("group:%s", group), article)
	}

	for _, group := range remove {
		conn.Do("SREM", fmt.Sprintf("group:%s", group), article)
	}
}

func GetGroupArticles(conn redis.Conn, page int, group, order string) ([]map[string]string, error) {
	key := order + group
	if res, _ := redis.Int(conn.Do("EXISTS", key)); res != 1 {
		conn.Do("ZINTERSTORE", key, 2, order, fmt.Sprintf("group:%s", group), "AGGREGATE", "MAX")
		conn.Do("EXPIRE", key, 60)
	}
	return GetArticles(conn, page, key)
}
