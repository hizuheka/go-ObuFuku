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

	// CRLF変換ライターを使用（改行コードの一貫性を保つため）
	writer := newCRLFWriter(outputFile)

	return processRemove(inputFile, writer, targetStr)
}

// processRemove は読み込んだテキストからターゲット文字列を削除して書き込みます。
func processRemove(r io.Reader, w io.Writer, targetStr string) error {
	reader := bufio.NewReader(r)

	for {
		// 行単位で読み込む
		// 注意: 行が非常に長い場合、メモリを多く消費する可能性がありますが、
		// 仕様上「改行されたテキストファイル」とあるため、標準的な行長を想定しています。
		line, err := reader.ReadString('\n')

		// 読み込んだ行（改行含む）からターゲット文字列を全て削除
		processedLine := strings.ReplaceAll(line, targetStr, "")

		// 書き込み
		if _, writeErr := w.Write([]byte(processedLine)); writeErr != nil {
			return fmt.Errorf("failed to write to output: %w", writeErr)
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("error reading input: %w", err)
		}
	}

	return nil
}
