package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
)

// bufferSize は一度に読み込むチャンクのサイズです。
// 巨大ファイルに対応するため、適度なサイズ（例: 64KB）にします。
const bufferSize = 64 * 1024

// NewlineProcessor は改行挿入処理の状態を保持します。
type NewlineProcessor struct {
	reader   io.Reader
	writer   io.Writer
	target   []byte
	position string // "before" or "after"
}

// runNewline はファイルI/Oをセットアップし、プロセッサを実行します。
func runNewline(targetStr, position, inputPath, outputPath string) error {
	if position != "before" && position != "after" {
		return fmt.Errorf("invalid position '%s': must be 'before' or 'after'", position)
	}
	if len(targetStr) == 0 {
		return fmt.Errorf("target string cannot be empty")
	}

	// 入力ファイル
	inputFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open input file: %w", err)
	}
	defer inputFile.Close()

	// 出力ファイル
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outputFile.Close()

	// CRLF変換ライターを使用（書き込むときは \n だけで済むようにする）
	crlfWriter := newCRLFWriter(outputFile)

	processor := newNewlineProcessor(inputFile, crlfWriter, targetStr, position)
	return processor.process()
}

// newNewlineProcessor はプロセッサを初期化します。
func newNewlineProcessor(r io.Reader, w io.Writer, targetStr, position string) *NewlineProcessor {
	return &NewlineProcessor{
		reader:   r,
		writer:   w,
		target:   []byte(targetStr),
		position: position,
	}
}

// process はストリーム処理を実行します。
func (p *NewlineProcessor) process() error {
	buf := make([]byte, bufferSize)
	var leftover []byte

	// 置換用のデータを作成
	var replacement []byte
	newline := []byte("\n") // crlfWriterが \r\n に変換してくれる
	if p.position == "before" {
		replacement = append(newline, p.target...)
	} else {
		replacement = append(p.target, newline...)
	}

	for {
		// バッファに読み込み
		n, err := p.reader.Read(buf)
		if n > 0 {
			// 前回の持ち越し分と結合
			data := append(leftover, buf[:n]...)

			// 結合したデータ内でターゲットを検索・置換して書き込み
			// 未処理の末尾（持ち越し分）を返す
			leftover, err = p.processChunk(data, replacement)
			if err != nil {
				return err
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading input: %w", err)
		}
	}

	// 最後に残った持ち越し分を書き込む
	if len(leftover) > 0 {
		if _, err := p.writer.Write(leftover); err != nil {
			return fmt.Errorf("error writing final bytes: %w", err)
		}
	}

	return nil
}

// processChunk は、データ内のターゲット文字列を置換して書き込み、
// 次回に持ち越すべき「末尾のデータ」を返します。
func (p *NewlineProcessor) processChunk(data, replacement []byte) ([]byte, error) {
	// bytes.Indexで検索し、見つかる限り置換して書き込む
	for {
		idx := bytes.Index(data, p.target)
		if idx == -1 {
			break
		}

		// マッチした箇所の「手前」まで書き込む
		if _, err := p.writer.Write(data[:idx]); err != nil {
			return nil, err
		}

		// 「置換後の文字列（改行付き）」を書き込む
		if _, err := p.writer.Write(replacement); err != nil {
			return nil, err
		}

		// 処理した部分をスライスから削除
		data = data[idx+len(p.target):]
	}

	// --- 持ち越し判定 ---
	// 残ったデータの末尾に、ターゲット文字列の一部が含まれている可能性があるため、
	// ターゲットの長さ - 1 バイト分は書き込まずに次へ持ち越す。

	keepLen := len(p.target) - 1
	if keepLen < 0 {
		keepLen = 0
	}

	if len(data) > keepLen {
		// 確定している部分（持ち越さなくて良い部分）を書き込む
		writeLen := len(data) - keepLen
		if _, err := p.writer.Write(data[:writeLen]); err != nil {
			return nil, err
		}
		// 残りを持ち越しとして返す
		return data[writeLen:], nil
	}

	// データ全体が短い場合は、全量を持ち越す
	return data, nil
}
