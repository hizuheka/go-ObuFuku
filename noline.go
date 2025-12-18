package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
)

// runNoLine はファイルの改行コードを除去します。
func runNoLine(inputPath, outputPath string) error {
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

	// 書き込みパフォーマンス向上のため bufio.Writer を使用
	// 注意: ここでは crlfWriter は使わない（改行を消すため）
	writer := bufio.NewWriter(outputFile)
	defer writer.Flush()

	return processNoLine(inputFile, writer)
}

// processNoLine は bufio.Scanner を使い、改行を除去して書き込みます。
// 前提: 入力ファイルの1行は巨大なサイズ（64KB以上）ではないこと。
func processNoLine(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)

	// Scannerはデフォルトで行単位で読み込み、末尾の改行コードを自動的に削除します。
	for scanner.Scan() {
		// 改行が削除された状態のデータ（行の中身）を取得
		line := scanner.Bytes()

		// そのまま書き込む（改行なしで連結される）
		if _, err := w.Write(line); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error scanning input: %w", err)
	}

	return nil
}
