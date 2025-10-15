package main

// Counter は、インクリメントする数値を管理します。
type Counter struct {
	current int
}

// Next はカウンターを1つ進めて、その新しい値を返します。
func (c *Counter) Next() int {
	c.current++
	return c.current
}

// NameReplaceRule は、タグ名を置換するためのルールです。
type NameReplaceRule struct {
	OldName string
	NewName string
}

// InsertBeforeRule は、動的なXML断片を挿入するためのルールです。
type InsertBeforeRule struct {
	TargetTag   string
	XMLTemplate string   // fmt.Sprintf 形式のテンプレート文字列
	Counter     *Counter // 値を生成するためのカウンター (nilも可)
}

// ValueReplaceFunc は、古い値を受け取って新しい値を返す関数の型を定義します。
type ValueReplaceFunc func(oldValue string) string

// ValueReplaceRule は、元の値を利用して値を置換するためのルールです。
type ValueReplaceRule struct {
	TargetTag       string
	ReplacementFunc ValueReplaceFunc // 静的な文字列の代わりに、変換関数を持つ
}
