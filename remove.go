package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// runRemove は指定された複数の文字列削除処理を実行します。
func runRemove(targets []string, inputPath, outputPath string) error {
	// バリデーション: 少なくとも1つは有効な文字列が必要
	validTargets := make([]string, 0, len(targets))
	for _, t := range targets {
		if len(t) > 0 {
			validTargets = append(validTargets, t)
		}
	}
	if len(validTargets) == 0 {
		return fmt.Errorf("target strings cannot be empty")
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

	return processRemove(inputFile, writer, validTargets)
}

// processRemove は読み込んだテキストから複数のターゲット文字列を削除して書き込みます。
func processRemove(r io.Reader, w io.Writer, targets []string) error {
	reader := bufio.NewReader(r)

	for {
		// ReadStringは、末尾の改行コード(\n または \r\n)を含んで返す
		// 注意: 行が非常に長い場合、メモリを多く消費する可能性がありますが、
		// 仕様上「改行されたテキストファイル」とあるため、標準的な行長を想定しています。
		// 重要: EOFの場合も、そこまで読み込んだデータ(line)と err=io.EOF が同時に返ります。
		line, err := reader.ReadString('\n')

		// 読み込んだ行に対して、指定されたターゲットを順番にすべて削除
		processedLine := line
		for _, target := range targets {
			processedLine = strings.ReplaceAll(processedLine, target, "")
		}

		// 書き込み
		// (EOFの場合もデータがあればここで書き出されます)
		if _, writeErr := w.Write([]byte(processedLine)); writeErr != nil {
			return fmt.Errorf("failed to write to output: %w", writeErr)
		}

		// その後でエラー（EOF含む）をチェックします
		if err != nil {
			if err == io.EOF {
				// EOFに到達したらループを抜ける（既に書き込み済み）
				break
			}
			return fmt.Errorf("error reading input: %w", err)
		}
	}

	return nil
}
