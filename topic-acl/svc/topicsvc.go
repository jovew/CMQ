package svc

import (
	"database/sql"
	"fmt"
	"github.com/sirupsen/logrus"
	"net"
	"strconv"
	"errors"
)

type TopicConf struct {
	Host string
	Port uint16
	Password string
	Username string
	Database string
}

type TopicSvc struct {
	Conf *TopicConf
}

func NewTopicConfig() *TopicConf {
	return &TopicConf{
		Host:     "",
		Port:     0,
		Password: "",
		Username: "",
		Database: "",
	}
}

func NewTopicSvc(conf *TopicConf) *TopicSvc {
	return &TopicSvc{
		Conf: conf,
	}
}

func (ds *TopicSvc) Start() error {
	logrus.Infof("start mysql server. host : %s, port : %d, database : %s, username : %s, password : %s",
		ds.Conf.Host, ds.Conf.Port, ds.Conf.Database, ds.Conf.Username, ds.Conf.Password)

	addr := net.JoinHostPort(ds.Conf.Host, strconv.Itoa(int(ds.Conf.Port)))
	// [username[:password]@][protocol[(address)]]/dbname[?param1=value1&...&paramN=valueN]
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s", ds.Conf.Username, ds.Conf.Password, addr, ds.Conf.Database)
	db, err := sql.Open("mysql", dsn)
	ctx.db = db
	if err != nil {
		logrus.Error("open mysql connection failed.")
	}
    return err
}

func (ds *TopicSvc) Stop() {
	ctx.db.Close()
}

const subTopicDatabase = "topic_subscription"

func (ds *TopicSvc) Subscribe(topic string, guid uint32, qos int) (uint32, error) {
	// query device from database
	queryStr := fmt.Sprintf("select id, product_key, delete_flag from %s where guid = %d and topic_filter = %s",
		subTopicDatabase, guid, topic)
	logrus.Infof("query string : %s", queryStr)
	var topicId uint32
	var productKey string
	var deleteFlag int32

	rows:= ctx.db.QueryRow(queryStr)
	err := rows.Scan(&topicId, &productKey, &deleteFlag)
	switch {
	case err == sql.ErrNoRows:
		// we should insert one record into database
		result, err := ctx.db.Exec(
			"INSERT INTO $1 (product_key, guid, topic_filter, qos, topic_type) VALUES ($2, $3, $4, $5, $6)",
			subTopicDatabase, productKey, guid, topic, qos, 0,
		)
		if err != nil {
			return 0, err
		} else {
			topicId, err := result.LastInsertId()
			return uint32(topicId), err
		}
	case err != nil:
		// database internal error
		return 0, errors.New("database insternal error.")
	default:
		// we should update the delete_flag to zero
		if deleteFlag != 0 {
			// update delete_flag to zero
			_, err := ctx.db.Exec(
				"UPDATE $1 set delete_flag=0 where guid=$2 and topic=$3",
				subTopicDatabase, guid, topic,
			)
			if err != nil {
				return 0, err
			} else {
				return topicId, nil
			}
		}
		return topicId, nil
	}
}

func (ds *TopicSvc) UnSubscribe(topic string, guid uint32) (int32, error) {
	result, err := ctx.db.Exec(
		"UPDATE $1 set delete_flag=1 where guid=$2 and topic=$3",
		subTopicDatabase, guid, topic,
	)
	if err != nil {
		return 0, err
	} else {
		rows, err := result.RowsAffected()
		return int32(rows), err
	}
}
