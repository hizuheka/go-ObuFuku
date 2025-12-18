package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestProcessRemove(t *testing.T) {
	testCases := []struct {
		name     string
		target   string
		input    string
		expected string
	}{
		{
			name:     "基本的な削除",
			target:   "foo",
			input:    "abc foo bar\nfoo\n",
			expected: "abc  bar\n\n",
		},
		{
			name:     "1行に複数回出現",
			target:   "AB",
			input:    "AB12AB34AB\n",
			expected: "1234\n",
		},
		{
			name:     "ターゲットが存在しない",
			target:   "xyz",
			input:    "hello world\n",
			expected: "hello world\n",
		},
		{
			name:     "ターゲットのみの行",
			target:   "DELETE_ME",
			input:    "DELETE_ME\nKeep\nDELETE_ME",
			expected: "\nKeep\n",
		},
		{
			name:     "空ファイル",
			target:   "A",
			input:    "",
			expected: "",
		},
		{
			name:     "日本語の削除",
			target:   "削除",
			input:    "これは削除対象です。\n残る文字。\n",
			expected: "これは対象です。\n残る文字。\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inputReader := strings.NewReader(tc.input)
			outputBuf := new(bytes.Buffer)

			// テストではCRLF変換を行わず、ロジックそのものを検証するため直接Bufferを渡す
			// (CRLF変換は newCRLFWriter で担保されているため)
			err := processRemove(inputReader, outputBuf, tc.target)
			if err != nil {
				t.Fatalf("processRemove failed: %v", err)
			}

			actual := outputBuf.String()
			if actual != tc.expected {
				t.Errorf("Expected %q, but got %q", tc.expected, actual)
			}
		})
	}
}
