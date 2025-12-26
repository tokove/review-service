package snowflake

import (
	"errors"
	"time"

	sf "github.com/bwmarrin/snowflake"
)

var (
	ErrInvalidInitParam  = errors.New("snowflake初始化失败，无效的startTime或machineID")
	ErrInvalidTimeFormat = errors.New("snowflake初始化失败，无效的startTime格式")
)

var node *sf.Node

// Init 初始化雪花算法
func Init(startTime string, machineID int64) (err error) {
	if len(startTime) == 0 || machineID <= 0 {
		return ErrInvalidInitParam
	}
	var st time.Time
	st, err = time.Parse("2006-01-02", startTime)
	if err != nil {
		return ErrInvalidTimeFormat
	}
	sf.Epoch = st.UnixNano() / 1000000
	node, err = sf.NewNode(machineID)
	return
}

// GenID 生成ID
func GenID() int64 {
	return node.Generate().Int64()
}
