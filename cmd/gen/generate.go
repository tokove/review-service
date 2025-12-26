package main

import (
	"flag"
	"fmt"
	"review-service/internal/conf"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"gorm.io/driver/mysql"
	"gorm.io/gen"
	"gorm.io/gorm"
)

// 生成 gen 代码

var flagconf string

func init() {
	flag.StringVar(&flagconf, "conf", "../../configs", "config path, eg: -conf config.yaml")
}

func connectDB(dsn string) *gorm.DB {
	db, err := gorm.Open(mysql.Open(dsn))
	if err != nil {
		panic(fmt.Errorf("connect db fail: %w", err))
	}
	return db
}

func main() {
	// 解析配置文件
	flag.Parse()

	c := config.New(
		config.WithSource(
			file.NewSource(flagconf),
		),
	)
	defer c.Close()

	if err := c.Load(); err != nil {
		panic(err)
	}

	var bc conf.Bootstrap
	if err := c.Scan(&bc); err != nil {
		panic(err)
	}

	// gen
	g := gen.NewGenerator(gen.Config{
		OutPath:       "../../internal/data/query", // 同时在dal下面生成model文件夹
		Mode:          gen.WithDefaultQuery | gen.WithQueryInterface,
		FieldNullable: true, // delete_at 可以为空就指针
	})

	g.UseDB(connectDB(bc.Data.Database.Source)) // connectDB 生成 *gorm.DB
	g.ApplyBasic(g.GenerateAllTable()...)
	g.Execute()
}
