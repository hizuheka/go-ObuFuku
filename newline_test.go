package main

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

// TestNewlineProcessor は改行挿入ロジックをテストします。
func TestNewlineProcessor(t *testing.T) {
	testCases := []struct {
		name      string
		target    string
		position  string
		input     string
		expected  string // \n は crlfWriterなしの場合。テストヘルパーで調整する
		chunkSize int    // テスト用に小さいバッファサイズを強制する（境界またぎを誘発するため）
	}{
		{
			name:      "単純な改行 (Before)",
			target:    "ITEM",
			position:  "before",
			input:     "AAITEMBB",
			expected:  "AA\nITEMBB",
			chunkSize: 1024,
		},
		{
			name:      "単純な改行 (After)",
			target:    "ITEM",
			position:  "after",
			input:     "AAITEMBB",
			expected:  "AAITEM\nBB",
			chunkSize: 1024,
		},
		{
			name:      "連続するターゲット",
			target:    "A",
			position:  "after",
			input:     "AAA",
			expected:  "A\nA\nA\n",
			chunkSize: 1024,
		},
		{
			name:     "*** 境界またぎ (Overlap) のテスト ***",
			target:   "TARGET",
			position: "before",
			// TARGETの長さは6。ChunkSizeを4にすることで、"TARG" | "ET" のように分割させる
			input:     "AAATARGETBBB",
			expected:  "AAA\nTARGETBBB",
			chunkSize: 4, // 非常に小さいバッファサイズ
		},
		{
			name:      "複数回の境界またぎ",
			target:    "XX",
			position:  "after",
			input:     "AXXBXXCXX",
			expected:  "AXX\nBXX\nCXX\n",
			chunkSize: 2, // 2バイトずつ読み込む
		},
		{
			name:      "ターゲットなし",
			target:    "XYZ",
			position:  "before",
			input:     "ABCDEFG",
			expected:  "ABCDEFG",
			chunkSize: 10,
		},
		{
			name:      "空ファイル",
			target:    "A",
			position:  "before",
			input:     "",
			expected:  "",
			chunkSize: 10,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inputReader := strings.NewReader(tc.input)
			outputBuf := new(bytes.Buffer)

			// テスト用に、NewlineProcessorを少しハックして小さいバッファで読み書きさせたいが、
			// NewlineProcessorのbufferSize定数は変更できない。
			// 代わりに、inputReaderを「指定サイズしか返さないReader」でラップする手法をとる。
			chunkedReader := &ChunkedReader{r: inputReader, size: tc.chunkSize}

			// crlfWriterは使わず、直接 \n を期待値としてテストする
			// (crlfWriterのロジック自体は writer.go で担保されているため)
			processor := newNewlineProcessor(chunkedReader, outputBuf, tc.target, tc.position)

			if err := processor.process(); err != nil {
				t.Fatalf("処理中にエラーが発生しました: %v", err)
			}

			actual := outputBuf.String()
			if actual != tc.expected {
				t.Errorf("期待値と一致しません。\nExpected: %q\nActual:   %q", tc.expected, actual)
			}
		})
	}
}

// ChunkedReader は、Read呼び出しごとに最大 size バイトしか返さないReaderです。
// ストリーム処理の境界テストに使用します。
type ChunkedReader struct {
	r    io.Reader
	size int
}

func (cr *ChunkedReader) Read(p []byte) (n int, err error) {
	if len(p) > cr.size {
		p = p[:cr.size]
	}
	return cr.r.Read(p)
}
