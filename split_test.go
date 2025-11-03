package main

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
)

// mockWriteCloser は、テスト用にbytes.Bufferをラップします
type mockWriteCloser struct {
	*bytes.Buffer
}

func (mwc *mockWriteCloser) Close() error {
	// 何もしない（bytes.BufferはClose不要）
	return nil
}

// TestSplitter はsplitロジックのテーブル駆動テストです
func TestSplitter(t *testing.T) {
	// 共通の入力XML
	const inputXML = `<?xml version="1.0" encoding="UTF-8"?>
<A>
  <B>
    <C><D>d1</D></C>
    <C><D>d2</D></C>
    <C><D>d3</D></C>
    <C><D>d4</D></C>
  </B>
</A>`

	// 期待されるヘッダーとフッター
	const header = `<?xml version="1.0" encoding="UTF-8"?>` + "\r\n" + "<A>\r\n  <B>\r\n"
	const footer = "\r\n  </B>\r\n</A>\r\n"

	// 期待されるCタグの断片
	c1 := "    <C>\r\n      <D>d1</D>\r\n    </C>"
	c2 := "    <C>\r\n      <D>d2</D>\r\n    </C>"
	c3 := "    <C>\r\n      <D>d3</D>\r\n    </C>"
	c4 := "    <C>\r\n      <D>d4</D>\r\n    </C>"

	testCases := []struct {
		name          string
		splitTag      string
		maxSize       int64
		expectedFiles map[int]string // 期待されるファイルの内容
		expectedErr   bool           // エラーを期待するか
	}{
		{
			name:     "1ファイル1タグ (maxSize 0)",
			splitTag: "C",
			maxSize:  0, // 0は「1タグ1ファイル」を意味する
			expectedFiles: map[int]string{
				1: header + c1 + footer,
				2: header + c2 + footer,
				3: header + c3 + footer,
				4: header + c4 + footer,
			},
		},
		{
			name:     "巨大なサイズ (全タグが1ファイル)",
			splitTag: "C",
			maxSize:  1024 * 1024, // 1MB (十分大きい)
			expectedFiles: map[int]string{
				1: header + c1 + "\r\n" + c2 + "\r\n" + c3 + "\r\n" + c4 + footer,
			},
		},
		{
			name:     "サイズベースの分割 (Cタグ2つ分)",
			splitTag: "C",
			maxSize:  int64(len(header) + len(c1) + len(c2) - 10), // Cタグ2つ強のサイズ
			expectedFiles: map[int]string{
				// 1ファイル目にc1, c2が入る (サイズチェックはc2の後)
				1: header + c1 + "\r\n" + c2 + footer,
				// 2ファイル目にc3, c4が入る
				2: header + c3 + "\r\n" + c4 + footer,
			},
		},
		{
			name:     "ルート要素での分割 (分割されない)",
			splitTag: "A",
			maxSize:  1,
			expectedFiles: map[int]string{
				1: header + c1 + "\r\n" + c2 + "\r\n" + c3 + "\r\n" + c4 + footer,
			},
		},
		{
			name:        "不正なXML",
			splitTag:    "C",
			maxSize:     1024,
			expectedErr: true, // runTestSplitterがエラーを返す
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// --- テスト用のモックファクトリとバッファを作成 ---
			buffers := make(map[int]*bytes.Buffer)
			factory := func(part int) (io.WriteCloser, error) {
				buf := new(bytes.Buffer)
				buffers[part] = buf
				return &mockWriteCloser{buf}, nil
			}

			// --- テスト対象のXMLを準備 ---
			xmlInput := inputXML
			if tc.name == "不正なXML" {
				xmlInput = `<A><B></A></B>`
			}

			// --- コアロジックの実行 ---
			s, err := newSplitter(strings.NewReader(xmlInput), factory, tc.splitTag, tc.maxSize)
			if err != nil && !tc.expectedErr {
				t.Fatalf("newSplitterで予期せぬエラー: %v", err)
			}

			err = s.process()

			fmt.Printf("maxsize=%d\n", tc.maxSize)
			// --- アサーション (結果の検証) ---
			if tc.expectedErr {
				if err == nil {
					t.Fatal("エラーが期待されましたが、nilが返りました")
				}
				return // エラー発生が期待通りなのでテスト終了
			}
			if err != nil {
				t.Fatalf("processで予期せぬエラー: %v", err)
			}

			// ファイル数のチェック
			if len(buffers) != len(tc.expectedFiles) {
				t.Fatalf("期待したファイル数 %d, 実際 %d", len(tc.expectedFiles), len(buffers))
			}

			// 各ファイルの内容をチェック
			for i, expectedContent := range tc.expectedFiles {
				actualContent, ok := buffers[i]
				if !ok {
					t.Fatalf("期待したファイル %d が生成されていません", i)
				}

				// TrimSpaceで末尾の改行を揃えて比較
				actual := strings.TrimSpace(actualContent.String())
				expected := strings.TrimSpace(expectedContent)

				if actual != expected {
					t.Errorf("ファイル %d の内容が期待と異なります。\n\n--- 期待した出力 (Expected) ---\n%s\n\n--- 実際の出力 (Actual) ---\n%s", i, expected, actual)
				}
			}
		})
	}
}

// TestSplitterFactoryError はファクトリがエラーを返すケースをテストします
func TestSplitterFactoryError(t *testing.T) {
	factoryErr := fmt.Errorf("ディスクがいっぱいです")

	factory := func(part int) (io.WriteCloser, error) {
		return nil, factoryErr
	}

	s, err := newSplitter(strings.NewReader(`<?xml version="1.0"?><root/>`), factory, "tag", 1024)
	if err != nil {
		t.Fatalf("newSplitterで予期せぬエラー: %v", err)
	}

	// 最初のファイルを開こうとしてエラーになる
	err = s.process()
	if err == nil || !strings.Contains(err.Error(), factoryErr.Error()) {
		t.Errorf("期待したエラー '%v' が返りませんでした, 実際のエラー: %v", factoryErr, err)
	}
}
