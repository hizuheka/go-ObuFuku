package main

import (
	"strings"
	"testing"
)

func TestMeasureMaxLine(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected int64
	}{
		{
			name:     "基本的なケース",
			input:    "abc\nde\nfghi\n",
			expected: 5, // "fghi\n" -> 5 bytes
		},
		{
			name:     "改行なしの1行",
			input:    "12345",
			expected: 5, // "12345" -> 5 bytes
		},
		{
			name:     "CRLF (Windows改行)",
			input:    "a\r\nbb\r\n",
			expected: 4, // "bb\r\n" -> 4 bytes
		},
		{
			name:     "空ファイル",
			input:    "",
			expected: 0,
		},
		{
			name:     "空行のみ",
			input:    "\n\n",
			expected: 1, // "\n" -> 1 byte
		},
		{
			name:     "末尾改行なし",
			input:    "abc\n123456",
			expected: 6, // "123456" -> 6 bytes (これが最大)
		},
		{
			name:     "マルチバイト文字 (UTF-8)",
			input:    "あいう\nえ\n", // "あいう" (9bytes) + "\n" (1byte) = 10bytes
			expected: 10,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := strings.NewReader(tc.input)
			actual, err := measureMaxLine(r)
			if err != nil {
				t.Fatalf("measureMaxLine failed: %v", err)
			}

			if actual != tc.expected {
				t.Errorf("Expected %d, but got %d (input: %q)", tc.expected, actual, tc.input)
			}
		})
	}
}

// TestMeasureMaxLine_HugeLine は、バッファサイズ(32KB)を超える長い行をテストします
func TestMeasureMaxLine_HugeLine(t *testing.T) {
	// 40KBの行を作成 ( 'A' * 40960 )
	longLine := strings.Repeat("A", 40*1024)
	input := longLine + "\nshort\n"

	expected := int64(len(longLine) + 1) // +1 for \n

	r := strings.NewReader(input)
	actual, err := measureMaxLine(r)
	if err != nil {
		t.Fatalf("measureMaxLine failed: %v", err)
	}

	if actual != expected {
		t.Errorf("Expected %d, but got %d", expected, actual)
	}
}
