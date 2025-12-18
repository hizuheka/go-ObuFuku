package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestProcessNoLine(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "LF改行の削除",
			input:    "abc\ndef\nghi",
			expected: "abcdefghi",
		},
		{
			name:     "CRLF改行の削除",
			input:    "abc\r\ndef\r\n",
			expected: "abcdef",
		},
		{
			name:     "CRのみの削除 (Classic Mac等)",
			input:    "abc\rdef",
			expected: "abc\rdef",
		},
		{
			name:     "改行なし",
			input:    "abcdef",
			expected: "abcdef",
		},
		{
			name:     "空ファイル",
			input:    "",
			expected: "",
		},
		{
			name:     "空行の連続",
			input:    "A\n\n\nB",
			expected: "AB",
		},
		{
			name:     "バッファ境界テスト (疑似的)",
			input:    strings.Repeat("A", 100) + "\n" + strings.Repeat("B", 100),
			expected: strings.Repeat("A", 100) + strings.Repeat("B", 100),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := strings.NewReader(tc.input)
			buf := new(bytes.Buffer)

			if err := processNoLine(r, buf); err != nil {
				t.Fatalf("processNoLine failed: %v", err)
			}

			if buf.String() != tc.expected {
				t.Errorf("Expected %q, but got %q", tc.expected, buf.String())
			}
		})
	}
}
