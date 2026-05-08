package data

import (
	"review-service/internal/conf"
	"review-service/internal/data/query"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(NewDB, NewES, NewData, NewReviewRepo)

// Data .
type Data struct {
	// TODO wrapped database client
	query *query.Query
	es    *elasticsearch.TypedClient
	log   *log.Helper
}

// NewData .
func NewData(db *gorm.DB, es *elasticsearch.TypedClient, logger log.Logger) (*Data, func(), error) {
	cleanup := func() {
		log.NewHelper(logger).Info("closing the data resources")
	}
	// 为 gen 生成的 query 设置 数据库连接对象
	query.SetDefault(db)
	return &Data{query: query.Q, es: es, log: log.NewHelper(logger)}, cleanup, nil
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
