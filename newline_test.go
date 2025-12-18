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
		targets   []string // 変更: string -> []string
		position  string
		input     string
		expected  string
		chunkSize int
	}{
		{
			name:      "単純な改行 (Before)",
			targets:   []string{"ITEM"},
			position:  "before",
			input:     "AAITEMBB",
			expected:  "AA\nITEMBB",
			chunkSize: 1024,
		},
		{
			name:      "単純な改行 (After)",
			targets:   []string{"ITEM"},
			position:  "after",
			input:     "AAITEMBB",
			expected:  "AAITEM\nBB",
			chunkSize: 1024,
		},
		{
			name:      "連続するターゲット",
			targets:   []string{"A"},
			position:  "after",
			input:     "AAA",
			expected:  "A\nA\nA\n",
			chunkSize: 1024,
		},
		{
			name:     "*** 境界またぎ (Overlap) のテスト ***",
			targets:  []string{"TARGET"},
			position: "before",
			// TARGETの長さは6。ChunkSizeを4にすることで、"TARG" | "ET" のように分割させる
			input:     "AAATARGETBBB",
			expected:  "AAA\nTARGETBBB",
			chunkSize: 4, // 非常に小さいバッファサイズ
		},
		{
			name:      "複数回の境界またぎ",
			targets:   []string{"XX"},
			position:  "after",
			input:     "AXXBXXCXX",
			expected:  "AXX\nBXX\nCXX\n",
			chunkSize: 2, // 2バイトずつ読み込む
		},
		{
			name:      "ターゲットなし",
			targets:   []string{"XYZ"},
			position:  "before",
			input:     "ABCDEFG",
			expected:  "ABCDEFG",
			chunkSize: 10,
		},
		{
			name:      "空ファイル",
			targets:   []string{"A"},
			position:  "before",
			input:     "",
			expected:  "",
			chunkSize: 10,
		},
		{
			name:      "複数ターゲット",
			targets:   []string{"A", "B"},
			position:  "after",
			input:     "xAxxByyAzz",
			expected:  "xA\nxxB\nyyA\nzz",
			chunkSize: 1024,
		},
		{
			name:      "複数ターゲット (出現順序が引数と逆)",
			targets:   []string{"SECOND", "FIRST"},
			position:  "before",
			input:     "aaFIRSTbbSECONDcc",
			expected:  "aa\nFIRSTbb\nSECONDcc",
			chunkSize: 1024,
		},
		{
			name:      "ターゲットの長さが異なる (Overlap)",
			targets:   []string{"LONG", "S"},
			position:  "before",
			input:     "aaaLONGbbbSccc",
			expected:  "aaa\nLONGbbb\nSccc",
			chunkSize: 4, // "LONG" (4文字) が境界で分割されるようにする
		},
		{
			name:      "ターゲット同士が接近している場合",
			targets:   []string{",", ";"},
			position:  "after",
			input:     "a,b;c,d",
			expected:  "a,\nb;\nc,\nd",
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
			processor := newNewlineProcessor(chunkedReader, outputBuf, tc.targets, tc.position)

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
