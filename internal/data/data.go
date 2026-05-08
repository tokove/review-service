package data

import (
	"context"
	"review-service/internal/conf"
	"review-service/internal/data/query"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(NewDB, NewES, NewRDB, NewData, NewReviewRepo)

// Data .
type Data struct {
	// TODO wrapped database client
	query *query.Query
	es    *elasticsearch.TypedClient
	rdb   *redis.Client
	log   *log.Helper
}

// NewData .
func NewData(db *gorm.DB, es *elasticsearch.TypedClient, rdb *redis.Client, logger log.Logger) (*Data, func(), error) {
	cleanup := func() {
		log.NewHelper(logger).Info("closing the data resources")
	}
	// 为 gen 生成的 query 设置 数据库连接对象
	query.SetDefault(db)
	return &Data{query: query.Q, es: es, rdb: rdb, log: log.NewHelper(logger)}, cleanup, nil
}

func NewDB(c *conf.Data) (*gorm.DB, error) {
	return gorm.Open(mysql.Open(c.Database.Source))
}

func NewES(c *conf.ElasticSearch) (*elasticsearch.TypedClient, error) {
	cfg := elasticsearch.Config{
		Addresses: c.GetAddresses(),
	}
	return elasticsearch.NewTypedClient(cfg)
}

func NewRDB(c *conf.Data) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         c.Redis.Addr,
		WriteTimeout: c.Redis.WriteTimeout.AsDuration(),
		ReadTimeout:  c.Redis.ReadTimeout.AsDuration(),
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}
	return rdb, nil
}
