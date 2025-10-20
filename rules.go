package main

import (
	"fmt"
)

// Counter は、インクリメントする数値を管理します。
type Counter struct {
	current int
}

func (c *Counter) Next() int {
	c.current++
	return c.current
}

// --- 実行時に使われるルール構造体 ---
type NameReplaceRule struct {
	OldName string
	NewName string
}
type InsertBeforeRule struct {
	TargetTag   string
	XMLTemplate string
	Counter     *Counter
}
type ValueReplaceFunc func(oldValue string) string
type ValueReplaceRule struct {
	TargetTag       string
	ReplacementFunc ValueReplaceFunc
}

// 子要素をラップするためのルール
type WrapRule struct {
	TargetTag  string
	WrapperTag string
}

// --- JSONファイルから読み込むための設定構造体 ---
type Config struct {
	NameRules   []ConfigNameRule         `json:"name_rules"`
	InsertRules []ConfigInsertRule       `json:"insert_rules"`
	ValueRules  []ConfigValueRule        `json:"value_rules"`
	WrapRules   []ConfigWrapRule         `json:"wrap_rules"`
	Counters    map[string]ConfigCounter `json:"counters"`
}

type ConfigNameRule struct {
	Old string `json:"old"`
	New string `json:"new"`
}
type ConfigInsertRule struct {
	Target   string `json:"target"`
	Template string `json:"template"`
	Counter  string `json:"counter"`
}
type ConfigValueRule struct {
	Target string                 `json:"target"`
	Type   string                 `json:"type"`
	Params map[string]interface{} `json:"params"`
}

type ConfigWrapRule struct {
	Target  string `json:"target"`
	Wrapper string `json:"wrapper"`
}
type ConfigCounter struct {
	Start int `json:"start"`
}

// buildValueReplaceFunc 関数
func buildValueReplaceFunc(rule ConfigValueRule) (ValueReplaceFunc, error) {
	switch rule.Type {
	case "prepend":
		prefix, ok := rule.Params["prefix"].(string)
		if !ok {
			return nil, fmt.Errorf("invalid or missing 'prefix' for prepend rule")
		}
		return func(oldValue string) string {
			return prefix + oldValue
		}, nil
	case "append":
		suffix, ok := rule.Params["suffix"].(string)
		if !ok {
			return nil, fmt.Errorf("invalid or missing 'suffix' for append rule")
		}
		return func(oldValue string) string {
			return oldValue + suffix
		}, nil
	default:
		return nil, fmt.Errorf("unknown value rule type: '%s'", rule.Type)
	}
}
