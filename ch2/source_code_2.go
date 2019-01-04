package ch2

import (
	"github.com/gomodule/redigo/redis"
	"time"
	"fmt"
)

//2.1
func CheckToken(conn redis.Conn,token string)(string,error){
	userId,err:=redis.String(conn.Do("HGET","login:",token))
	if err!=nil{
		return "",err
	}
	return userId,nil
}

func UpdateToken(conn redis.Conn,token string,user string,item string){
	timestamp :=time.Now().Unix()
	conn.Do("HSET","login:",token,user)
	conn.Do("ZADD","recent:",timestamp,token)
	if item!=""{
		conn.Do("ZADD",fmt.Sprintf("viewed:%s",token),timestamp,item)
		conn.Do("ZREMRANGEBYRANK",fmt.Sprintf("viewed:%s",token),0,-26)
	}
}

//2.2
