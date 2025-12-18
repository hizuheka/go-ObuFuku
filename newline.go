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
	reader       io.Reader
	writer       io.Writer
	targets      [][]byte // 複数のターゲットをバイト列で保持
	maxTargetLen int      // 最も長いターゲットの長さ（持ち越し計算用）
	position     string
}

// runNewline はファイルI/Oをセットアップし、プロセッサを実行します。
func runNewline(targetStrs []string, position, inputPath, outputPath string) error {
	if position != "before" && position != "after" {
		return fmt.Errorf("invalid position '%s': must be 'before' or 'after'", position)
	}
	if len(targetStrs) == 0 {
		return fmt.Errorf("target strings cannot be empty")
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

	processor := newNewlineProcessor(inputFile, crlfWriter, targetStrs, position)
	return processor.process()
}

// newNewlineProcessor はプロセッサを初期化します。
func newNewlineProcessor(r io.Reader, w io.Writer, targetStrs []string, position string) *NewlineProcessor {
	var targets [][]byte
	maxLen := 0

	// 文字列スライスをバイト列スライスに変換しつつ、最大長を計算
	for _, s := range targetStrs {
		if len(s) == 0 {
			continue
		}
		b := []byte(s)
		targets = append(targets, b)
		if len(b) > maxLen {
			maxLen = len(b)
		}
	}

	return &NewlineProcessor{
		reader:       r,
		writer:       w,
		targets:      targets,
		maxTargetLen: maxLen,
		position:     position,
	}
}

// process はストリーム処理を実行します。
func (p *NewlineProcessor) process() error {
	buf := make([]byte, bufferSize)
	var leftover []byte
	newline := []byte("\n")

	for {
		// バッファに読み込み
		n, err := p.reader.Read(buf)
		if n > 0 {
			// 前回の持ち越し分と結合
			data := append(leftover, buf[:n]...)

			// バッファ内の検索と置換
			// 戻り値として「次に持ち越すべきデータ」を受け取る
			leftover, err = p.processChunk(data, newline)
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

// processChunk は複数のターゲットを検索し、見つかった順に処理します
func (p *NewlineProcessor) processChunk(data, newline []byte) ([]byte, error) {
	for {
		// 複数のターゲットの中で、最も「手前（小さいインデックス）」にあるものを探す
		bestIdx := -1
		var bestTarget []byte

		for _, t := range p.targets {
			idx := bytes.Index(data, t)
			if idx != -1 {
				// まだ見つかっていない、または、より手前にある場合
				if bestIdx == -1 || idx < bestIdx {
					bestIdx = idx
					bestTarget = t
				}
			}
		}

		// 何も見つからなければループ終了
		if bestIdx == -1 {
			break
		}

		// マッチした箇所の「手前」まで書き込む
		if _, err := p.writer.Write(data[:bestIdx]); err != nil {
			return nil, err
		}

		// ターゲット文字列と改行を書き込む
		if p.position == "before" {
			// 改行 + ターゲット
			if _, err := p.writer.Write(newline); err != nil {
				return nil, err
			}
			if _, err := p.writer.Write(bestTarget); err != nil {
				return nil, err
			}
		} else {
			// ターゲット + 改行
			if _, err := p.writer.Write(bestTarget); err != nil {
				return nil, err
			}
			if _, err := p.writer.Write(newline); err != nil {
				return nil, err
			}
		}

		// 処理した部分（ターゲット含む）までをデータから削除して次へ
		data = data[bestIdx+len(bestTarget):]
	}

	// --- 持ち越し判定 ---
	// 残ったデータの末尾に、ターゲット文字列の一部が含まれている可能性があるため、
	// ターゲットの中で「最も長いもの」の長さ - 1 バイト分を持ち越す
	keepLen := p.maxTargetLen - 1
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
