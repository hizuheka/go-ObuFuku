package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// runRemove は指定された文字列を削除処理を実行します。
func runRemove(targetStr, inputPath, outputPath string) error {
	if len(targetStr) == 0 {
		return fmt.Errorf("target string cannot be empty")
	}

	inputFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open input file: %w", err)
	}
	defer inputFile.Close()

	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outputFile.Close()

	// CRLF変換ライターを使うと、元の \r\n が \r\r\n になってしまうため、
	// 通常のバッファ付きWriterを使用する（入力の改行コードをそのまま維持する）。
	writer := bufio.NewWriter(outputFile)
	defer writer.Flush() // 忘れずにFlushする

	return processRemove(inputFile, writer, targetStr)
}

// processRemove は読み込んだテキストからターゲット文字列を削除して書き込みます。
func processRemove(r io.Reader, w io.Writer, targetStr string) error {
	reader := bufio.NewReader(r)

	for {
		// ReadStringは、末尾の改行コード(\n または \r\n)を含んで返す
		// 注意: 行が非常に長い場合、メモリを多く消費する可能性がありますが、
		// 仕様上「改行されたテキストファイル」とあるため、標準的な行長を想定しています。
		// 重要: EOFの場合も、そこまで読み込んだデータ(line)と err=io.EOF が同時に返ります。
		line, err := reader.ReadString('\n')

		// 1. まず、読み込めた分を処理して書き出します
		// (EOFの場合もデータがあればここで書き出されます)
		processedLine := strings.ReplaceAll(line, targetStr, "")

		// 書き込み
		if _, writeErr := w.Write([]byte(processedLine)); writeErr != nil {
			return fmt.Errorf("failed to write to output: %w", writeErr)
		}

		// 2. その後でエラー（EOF含む）をチェックします
		if err != nil {
			if err == io.EOF {
				// 修正ポイント:
				// 既に上で write しているので、ここでは何もせずループを抜けるだけでOK。
				// 以前のコードはここで再度 write していたため重複していました。
				break
			}
			return fmt.Errorf("error reading input: %w", err)
		}
	}

	return nil
}
