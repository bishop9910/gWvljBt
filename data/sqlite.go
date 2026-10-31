package data

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Store 是所有服务共享的持久化入口，包一层 gorm.DB 方便以后加事务工具函数。
type Store struct {
	DB *gorm.DB
}

// Open 打开（不存在则创建）SQLite 数据库，并自动迁移表结构。
//
// 数据库只放"数据"：游戏、补丁、快照。配置项仍然留在 config.json 里，
// 因为它是纯 KV、需要人手可编辑，也没必要为几个开关引入一张表。
//
// 驱动选的是 github.com/glebarez/sqlite（纯 Go，底层 modernc.org/sqlite），
// 不是 cgo 版的 mattn/go-sqlite3：这样 CGO_ENABLED=0 也能正常跑，
// 不受 Wails 构建默认值和本机有没有 C 编译器影响。
func Open(path string) (*Store, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	// pragma 走驱动约定的 `_pragma=name(value)` 形式（每个新连接都会应用）：
	//   busy_timeout：并发写时排队等待而不是直接抛 database is locked；
	//   journal_mode=WAL：读写可以并存，滚动列表时不会卡住写入；
	//   foreign_keys=1：让 ON DELETE CASCADE 真正生效。
	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		SkipDefaultTransaction: true,
		Logger:                 logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败 %s: %w", path, err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取连接池失败: %w", err)
	}
	// SQLite 只有一个写者，单连接可以彻底避开锁竞争；桌面应用的并发量
	// 远低于这个瓶颈，换来的是"永远不会 database is locked"。
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	if err := db.AutoMigrate(&Game{}, &Patch{}, &PatchFile{}, &Snapshot{}); err != nil {
		return nil, fmt.Errorf("迁移数据库结构失败: %w", err)
	}

	return &Store{DB: db}, nil
}

// Close 关闭底层连接，进程退出前调用。
func (s *Store) Close() error {
	if s == nil || s.DB == nil {
		return nil
	}
	sqlDB, err := s.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
