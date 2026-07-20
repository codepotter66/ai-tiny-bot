package store

import "github.com/google/uuid"

// NewID 返回一个 UUID v4 字符串作为主键。
func NewID() string { return uuid.NewString() }

// newID 保留内部别名。
func newID() string { return NewID() }
