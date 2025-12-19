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
			expected:  "AA\r\nITEMBB",
			chunkSize: 1024,
		},
		{
			name:      "単純な改行 (After)",
			targets:   []string{"ITEM"},
			position:  "after",
			input:     "AAITEMBB",
			expected:  "AAITEM\r\nBB",
			chunkSize: 1024,
		},
		{
			name:      "連続するターゲット",
			targets:   []string{"A"},
			position:  "after",
			input:     "AAA",
			expected:  "A\r\nA\r\nA\r\n",
			chunkSize: 1024,
		},
		{
			name:     "*** 境界またぎ (Overlap) のテスト ***",
			targets:  []string{"TARGET"},
			position: "before",
			// TARGETの長さは6。ChunkSizeを4にすることで、"TARG" | "ET" のように分割させる
			input:     "AAATARGETBBB",
			expected:  "AAA\r\nTARGETBBB",
			chunkSize: 4, // 非常に小さいバッファサイズ
		},
		{
			name:      "複数回の境界またぎ",
			targets:   []string{"XX"},
			position:  "after",
			input:     "AXXBXXCXX",
			expected:  "AXX\r\nBXX\r\nCXX\r\n",
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
			expected:  "xA\r\nxxB\r\nyyA\r\nzz",
			chunkSize: 1024,
		},
		{
			name:      "複数ターゲット (出現順序が引数と逆)",
			targets:   []string{"SECOND", "FIRST"},
			position:  "before",
			input:     "aaFIRSTbbSECONDcc",
			expected:  "aa\r\nFIRSTbb\r\nSECONDcc",
			chunkSize: 1024,
		},
		{
			name:      "ターゲットの長さが異なる (Overlap)",
			targets:   []string{"LONG", "S"},
			position:  "before",
			input:     "aaaLONGbbbSccc",
			expected:  "aaa\r\nLONGbbb\r\nSccc",
			chunkSize: 4, // "LONG" (4文字) が境界で分割されるようにする
		},
		{
			name:      "ターゲット同士が接近している場合",
			targets:   []string{",", ";"},
			position:  "after",
			input:     "a,b;c,d",
			expected:  "a,\r\nb;\r\nc,\r\nd",
			chunkSize: 10,
		},
		{
			// 修正: 挿入される改行は \r\n になったため、期待値も合わせる
			name:      "単一ターゲット",
			targets:   []string{"ITEM"},
			position:  "before",
			input:     "AAITEMBB",
			expected:  "AA\r\nITEMBB",
			chunkSize: 1024,
		},
		{
			// 修正: 挿入される改行は \r\n
			name:      "複数ターゲット",
			targets:   []string{"A", "B"},
			position:  "after",
			input:     "xAxxByyAzz",
			expected:  "xA\r\nxxB\r\nyyA\r\nzz",
			chunkSize: 1024,
		},
		{
			// 修正: 挿入される改行は \r\n
			name:      "ターゲットの長さが異なる (Overlap)",
			targets:   []string{"LONG", "S"},
			position:  "before",
			input:     "aaaLONGbbbSccc",
			expected:  "aaa\r\nLONGbbb\r\nSccc",
			chunkSize: 4,
		},

		// *** 新規追加: 元のCRLFが維持されるか確認するケース ***
		{
			name:     "入力に含まれるCRLFが二重化しないか確認",
			targets:  []string{"TARGET"},
			position: "before",
			// 入力: 既に \r\n が含まれている
			input: "Line1\r\nTARGET\r\nLine2",
			// 期待:
			// Line1\r\n  -> そのまま (二重化していないこと)
			// \r\nTARGET -> before指定で挿入されたCRLF + TARGET
			// \r\nLine2  -> そのまま
			expected:  "Line1\r\n\r\nTARGET\r\nLine2",
			chunkSize: 1024,
		},
		{
			name:     "入力に含まれるCRLFが二重化しないか確認 (after)",
			targets:  []string{"TARGET"},
			position: "after",
			input:    "TARGET\r\nNext",
			// TARGET\r\n -> 挿入されたCRLF
			// \r\nNext   -> 元のCRLF (そのまま)
			expected:  "TARGET\r\n\r\nNext",
			chunkSize: 1024,
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
