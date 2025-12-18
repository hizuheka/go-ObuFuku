package main

import (
	"os"
	"testing"
)

func TestRunSort(t *testing.T) {
	// テスト用データ（順不同）
	inputData := "Banana\nApple\nCherry\nDate\nElderberry\nFig\nGrape"
	expectedData := "Apple\r\nBanana\r\nCherry\r\nDate\r\nElderberry\r\nFig\r\nGrape\r\n"

	// 入力ファイル作成
	tmpInput, err := os.CreateTemp("", "test_input_*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpInput.Name())
	tmpInput.WriteString(inputData)
	tmpInput.Close()

	// 出力ファイルパス
	tmpOutput := tmpInput.Name() + ".out"
	defer os.Remove(tmpOutput)

	// 実行
	// テスト時はメモリ制限を小さくしても動作することを確認（例: 10MB）
	memoryLimit := int64(10 * 1024 * 1024)
	if err := runSort(tmpInput.Name(), tmpOutput, memoryLimit); err != nil {
		t.Fatalf("runSort failed: %v", err)
	}

	// 結果確認
	content, err := os.ReadFile(tmpOutput)
	if err != nil {
		t.Fatal(err)
	}

	actual := string(content)
	if actual != expectedData {
		t.Errorf("Sort result mismatch.\nExpected:\n%q\nActual:\n%q", expectedData, actual)
	}
}
