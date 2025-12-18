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

// processNoLine はストリームから改行コード(\r, \n)を除去して書き込みます。
func processNoLine(r io.Reader, w io.Writer) error {
	buf := make([]byte, 32*1024) // 32KBバッファ

	for {
		n, err := r.Read(buf)
		if n > 0 {
			data := buf[:n]

			// 高速化ロジック:
			// 1バイトずつWriteすると遅いため、改行以外の「連続した区間」を探してまとめて書き込む
			start := 0
			for i := 0; i < n; i++ {
				if data[i] == '\r' || data[i] == '\n' {
					// 改行文字が見つかったら、そこまでの有効な区間を書き込む
					if i > start {
						if _, err := w.Write(data[start:i]); err != nil {
							return err
						}
					}
					// start位置を改行文字の次の文字へ移動（スキップ）
					start = i + 1
				}
			}

			// バッファの末尾まで改行がなかった場合、残りを書き込む
			if start < n {
				if _, err := w.Write(data[start:n]); err != nil {
					return err
				}
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading input: %w", err)
		}
	}

	return nil
}
