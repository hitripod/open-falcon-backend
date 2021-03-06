package redis

import (
	"encoding/json"
	"github.com/Cepave/open-falcon-backend/modules/sender/model"
	"github.com/garyburd/redigo/redis"
	log "github.com/sirupsen/logrus"
)

func PopAllSms(queue string) []*model.Sms {
	ret := []*model.Sms{}

	rc := ConnPool.Get()
	defer rc.Close()

	for {
		reply, err := redis.String(rc.Do("RPOP", queue))
		if err != nil {
			if err != redis.ErrNil {
				log.Println(err)
			}
			break
		}

		if reply == "" || reply == "nil" {
			continue
		}

		var sms model.Sms
		err = json.Unmarshal([]byte(reply), &sms)
		if err != nil {
			log.Println(err, reply)
			continue
		}

		ret = append(ret, &sms)
	}

	return ret
}

func PopAllMail(queue string) []*model.Mail {
	ret := []*model.Mail{}

	rc := ConnPool.Get()
	defer rc.Close()

	for {
		reply, err := redis.String(rc.Do("RPOP", queue))
		if err != nil {
			if err != redis.ErrNil {
				log.Println(err)
			}
			break
		}

		if reply == "" || reply == "nil" {
			continue
		}

		var mail model.Mail
		err = json.Unmarshal([]byte(reply), &mail)
		if err != nil {
			log.Println(err, reply)
			continue
		}

		ret = append(ret, &mail)
	}

	return ret
}

func PopAllQQ(queue string) []*model.QQ {
	ret := []*model.QQ{}

	rc := ConnPool.Get()
	defer rc.Close()

	for {
		reply, err := redis.String(rc.Do("RPOP", queue))
		if err != nil {
			if err != redis.ErrNil {
				log.Println(err)
			}
			break
		}

		if reply == "" || reply == "nil" {
			continue
		}

		var qq model.QQ
		err = json.Unmarshal([]byte(reply), &qq)
		if err != nil {
			log.Println(err, reply)
			continue
		}

		ret = append(ret, &qq)
	}
	return ret
}

func PopAllServerchan(queue string) []*model.Serverchan {
	ret := []*model.Serverchan{}

	rc := ConnPool.Get()
	defer rc.Close()

	for {
		reply, err := redis.String(rc.Do("RPOP", queue))
		if err != nil {
			if err != redis.ErrNil {
				log.Println(err)
			}
			break
		}

		if reply == "" || reply == "nil" {
			continue
		}

		var serverchan model.Serverchan
		err = json.Unmarshal([]byte(reply), &serverchan)
		if err != nil {
			log.Println(err, reply)
			continue
		}

		ret = append(ret, &serverchan)
	}
	return ret
}
