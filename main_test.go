package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// runTestProcessor は、テスト専用のヘルパー関数です。(変更なし)
// JSONルールと入力XMLを文字列として受け取り、変換後のXML文字列を返します。
func runTestProcessor(rulesJSON, inputXML string) (string, error) {
	// --- 1. ルールをJSON文字列から読み込む ---
	var config Config
	if err := json.Unmarshal([]byte(rulesJSON), &config); err != nil {
		return "", err
	}

	// --- 2. transform.go と同じルール組み立てロジック ---
	counters := make(map[string]*Counter)
	for name, counterConfig := range config.Counters {
		counters[name] = &Counter{current: counterConfig.Start}
	}
	var nameRules []NameReplaceRule
	for _, r := range config.NameRules {
		nameRules = append(nameRules, NameReplaceRule{OldName: r.Old, NewName: r.New})
	}
	var insertRules []InsertBeforeRule
	for _, r := range config.InsertRules {
		insertRules = append(insertRules, InsertBeforeRule{
			TargetTag:   r.Target,
			XMLTemplate: r.Template,
			Counter:     counters[r.Counter],
		})
	}
	var insertAfterRules []InsertBeforeRule
	for _, r := range config.InsertAfterRules {
		insertAfterRules = append(insertAfterRules, InsertBeforeRule{
			TargetTag:   r.Target,
			XMLTemplate: r.Template,
			Counter:     counters[r.Counter],
		})
	}
	var prependChildRules []InsertBeforeRule
	for _, r := range config.PrependChildRules {
		prependChildRules = append(prependChildRules, InsertBeforeRule{
			TargetTag:   r.Target,
			XMLTemplate: r.Template,
			Counter:     counters[r.Counter],
		})
	}
	var valueRules []ValueReplaceRule
	for _, r := range config.ValueRules {
		replaceFunc, err := buildValueReplaceFunc(r)
		if err != nil {
			return "", err
		}
		valueRules = append(valueRules, ValueReplaceRule{
			TargetTag:       r.Target,
			ReplacementFunc: replaceFunc,
		})
	}
	var wrapRules []WrapRule
	for _, r := range config.WrapRules {
		wrapRules = append(wrapRules, WrapRule{TargetTag: r.Target, WrapperTag: r.Wrapper})
	}
	var cdataRules []CdataRule
	for _, r := range config.CdataRules {
		cdataRules = append(cdataRules, CdataRule{Old: r.Old, New: r.New})
	}
	rawTags := config.RawTags
	deleteTags := config.DeleteTags

	// --- 3. メモリ上でプロセッサを実行 ---
	inputReader := strings.NewReader(inputXML)
	var outputBuffer bytes.Buffer
	writer := newCRLFWriter(&outputBuffer) // CRLFライターを使用

	proc := newProcessor(inputReader, writer, nameRules, insertRules, insertAfterRules, prependChildRules, valueRules, wrapRules, cdataRules, rawTags, deleteTags)
	if err := proc.Run(); err != nil {
		return "", err
	}

	return outputBuffer.String(), nil
}

// *** 新設: 正規化ヘルパー関数 ***
// normalizeXMLString は、テスト比較用にXML文字列を正規化します。
// インデント用のスペースと改行コードをすべて削除します。
func normalizeXMLString(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "")
	s = strings.ReplaceAll(s, "\n", "") // 念のためLFも
	s = strings.ReplaceAll(s, "  ", "") // 2-space indent
	s = strings.TrimSpace(s)
	return s
}

// TestTransformations は、全ての変換ルールを網羅的にテストします。
func TestTransformations(t *testing.T) {

	// --- *** 変更点: 期待値をインデントなしの1行に変更 *** ---
	// プログラムは <?xml ...?> を自動で出力するため、期待値にも含めます。
	testCases := []struct {
		name        string // テストケース名
		rulesJSON   string // このテストで使うルール
		inputXML    string // 入力XML
		expectedXML string // 期待する出力XML（正規化前）
	}{
		{
			name:        "タグ名置換",
			rulesJSON:   `{"name_rules": [{"old": "root", "new": "new_root"}]}`,
			inputXML:    `<root><item>1</item></root>`,
			expectedXML: `<new_root><item>1</item></new_root>`,
		},
		{
			name: "値置換 (prepend + remove_string)",
			rulesJSON: `{
				"value_rules": [
					{"target": "id", "type": "prepend", "params": {"prefix": "ID-"}},
					{"target": "note", "type": "remove_string", "params": {"remove": "OLD: "}}
				]
			}`,
			inputXML:    `<data><id>123</id><note>OLD: text</note></data>`,
			expectedXML: `<data><id>ID-123</id><note>text</note></data>`,
		},
		{
			name:        "タグ削除",
			rulesJSON:   `{"delete_tags": ["trash"]}`,
			inputXML:    `<root><item>keep</item><trash><data>delete</data></trash></root>`,
			expectedXML: `<root><item>keep</item></root>`,
		},
		{
			name: "後方挿入 (insert_after_rules) カウンター付き",
			rulesJSON: `{
				"insert_after_rules": [{"target": "item", "template": "<count>%d</count>", "counter": "c"}],
				"counters": {"c": {"start": 0}}
			}`,
			inputXML:    `<root><item>A</item><item>B</item></root>`,
			expectedXML: `<root><item>A</item><count>1</count><item>B</item><count>2</count></root>`,
		},
		{
			name: "子の先頭に挿入 (prepend_child_rules)",
			rulesJSON: `{
				"prepend_child_rules": [{"target": "item", "template": "<first></first>"}]
			}`,
			inputXML:    `<root><item><name>A</name></item></root>`,
			expectedXML: `<root><item><first></first><name>A</name></item></root>`,
		},
		{
			name: "子のラップ (wrap_rules)",
			rulesJSON: `{
				"wrap_rules": [{"target": "item", "wrapper": "content"}]
			}`,
			inputXML:    `<root><item><name>A</name><id>1</id></item></root>`,
			expectedXML: `<root><item><content><name>A</name><id>1</id></content></item></root>`,
		},
		{
			name: "Rawタグ処理 (raw_tags + cdata_rules)",
			rulesJSON: `{
				"raw_tags": ["data"],
				"cdata_rules": [{"old": "&", "new": "and"}]
			}`,
			inputXML:    `<root><data><![CDATA[A < B & C]]></data><safe>A = B</safe></root>`,
			expectedXML: `<root><data><![CDATA[A < B and C]]></data><safe>A = B</safe></root>`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// テストヘルパーを実行
			actualXML, err := runTestProcessor(tc.rulesJSON, tc.inputXML)
			if err != nil {
				t.Fatalf("処理中にエラーが発生しました: %v", err)
			}

			// --- *** 変更点: 正規化してから比較 *** ---
			actual := normalizeXMLString(actualXML)
			expected := normalizeXMLString(tc.expectedXML)

			if actual != expected {
				t.Errorf("期待する出力と一致しませんでした。\n\n--- 期待した出力 (Expected) ---\n%s\n\n--- 実際の出力 (Actual) ---\n%s", expected, actual)
			}
		})
	}
}
