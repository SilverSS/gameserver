package main

import (
	"context"
	"fmt"
	"log"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"
	"github.com/SilverSS/gameserver/ent"
	_ "github.com/lib/pq"
)

// DB 연결 및 마이그레이션 함수
func InitDB() (*ent.Client, error) {
	// 환경변수 또는 설정 파일로 분리 가능
	dsn := "host=localhost port=21483 user=eos password=SYRius214!@ dbname=gameserverdb sslmode=disable"
	drv, err := sql.Open(dialect.Postgres, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed opening connection to postgres: %w", err)
	}
	client := ent.NewClient(ent.Driver(drv))
	// 마이그레이션 실행
	if err := client.Schema.Create(context.Background()); err != nil {
		return nil, fmt.Errorf("failed creating schema resources: %w", err)
	}
	log.Println("DB 연결 및 마이그레이션 성공!")
	return client, nil
}
