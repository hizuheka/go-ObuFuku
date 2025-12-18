package main

import (
	"bufio"
	"container/heap"
	"context"
	"fmt"
	"os"
	"runtime"
	"slices"
	"sync"

	"golang.org/x/sync/errgroup"
)

// chunkPool はスライスを再利用してGC負荷を下げるためのプールです。
var chunkPool = sync.Pool{
	New: func() interface{} {
		// 初期容量として1万行分を確保
		return make([]string, 0, 10000)
	},
}

// runSort は並列外部マージソートを実行します。
// maxMemoryBytes: 使用するメモリ総量の上限 (Byte)
func runSort(inputPath, outputPath string, maxMemoryBytes int64) error {
	// 1. 並列分割ソートフェーズ: 一時ファイルを作成
	tempFiles, err := createSortedChunksParallel(inputPath, maxMemoryBytes)
	if err != nil {
		return err
	}

	// 処理終了後に一時ファイルを削除
	defer func() {
		for _, f := range tempFiles {
			f.Close()
			os.Remove(f.Name())
		}
	}()

	// 2. マージフェーズ: 一時ファイルを統合
	return mergeChunks(tempFiles, outputPath)
}

// createSortedChunksParallel は入力を並列に読み込み・ソートし、一時ファイル群を作成します。
func createSortedChunksParallel(inputPath string, maxMemoryBytes int64) ([]*os.File, error) {
	inputFile, err := os.Open(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open input: %w", err)
	}
	defer inputFile.Close()

	numWorkers := runtime.NumCPU()

	// 1ワーカーあたりのメモリ割り当て計算
	chunkSizeLimit := maxMemoryBytes / int64(numWorkers+1)
	if chunkSizeLimit < 1*1024*1024 {
		chunkSizeLimit = 1 * 1024 * 1024
	}

	// ジョブと結果のチャネル
	jobs := make(chan []string, numWorkers)
	results := make(chan string, numWorkers*10)

	// Context付きのErrGroupを作成
	// 誰かがエラーを返すとctxがキャンセルされ、全員に通知される
	g, ctx := errgroup.WithContext(context.Background())

	// --- Workers (Sorter & Writer) ---
	// Go 1.22+: 整数でのrangeループ
	for range numWorkers {
		g.Go(func() error {
			for lines := range jobs {
				// Contextチェック（他の場所でエラーが起きていないか）
				if ctx.Err() != nil {
					return ctx.Err()
				}

				// Go 1.21+: slices.Sort (高速かつジェネリクス対応)
				slices.Sort(lines)

				tmpPath, err := writeTempFileAndClose(lines)

				// 使い終わったスライスをプールへ返却（リセットして再利用）
				lines = lines[:0]
				chunkPool.Put(lines)

				if err != nil {
					return err
				}

				// 結果送信（ブロック対策としてselectでContextも監視）
				select {
				case results <- tmpPath:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return nil
		})
	}

	// --- Reader (Input Producer) ---
	g.Go(func() error {
		defer close(jobs) // 読み終わったら必ず閉じる

		scanner := bufio.NewScanner(inputFile)
		// 巨大な行に対応するためバッファを拡張
		buf := make([]byte, 1024*1024)
		scanner.Buffer(buf, 10*1024*1024)

		lines := chunkPool.Get().([]string)
		currentSize := int64(0)

		for scanner.Scan() {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			text := scanner.Text()
			lines = append(lines, text)
			currentSize += int64(len(text))

			// 指定サイズを超えたらジョブ投入
			if currentSize >= chunkSizeLimit {
				select {
				case jobs <- lines:
					// 次のバッファをプールから取得
					lines = chunkPool.Get().([]string)
					currentSize = 0
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}

		// 残りのデータを投入
		if len(lines) > 0 {
			select {
			case jobs <- lines:
			case <-ctx.Done():
				return ctx.Err()
			}
		} else {
			// データがない場合はプールに戻す
			chunkPool.Put(lines)
		}

		if err := scanner.Err(); err != nil {
			return fmt.Errorf("error reading input: %w", err)
		}
		return nil
	})

	// --- Result Collector ---
	var tempPaths []string

	// 結果回収用のサブゴルーチン
	// エラーに関わらず、全てのタスク終了後にresultsを閉じる役割
	go func() {
		g.Wait()
		close(results)
	}()

	// 結果を集める
	for path := range results {
		tempPaths = append(tempPaths, path)
	}

	// 最終的なエラーチェック
	if err := g.Wait(); err != nil {
		return nil, err
	}

	// ファイルパスからファイルオブジェクトを開いて返す
	var fileObjs []*os.File
	for _, path := range tempPaths {
		f, err := os.Open(path)
		if err != nil {
			// 既に開いたものは閉じる必要があるが、呼び出し元のdeferで処理されるためエラーを返すだけで良い
			return nil, err
		}
		fileObjs = append(fileObjs, f)
	}

	return fileObjs, nil
}

// writeTempFileAndClose はメモリ上の行を一時ファイルに書き出し、パスを返します。
func writeTempFileAndClose(lines []string) (string, error) {
	tmpFile, err := os.CreateTemp("", "sort-chunk-*.txt")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tmpFile.Close()

	w := bufio.NewWriter(tmpFile)
	for _, line := range lines {
		// 文字列連結を避けて書き込む (メモリ割り当て削減)
		if _, err := w.WriteString(line); err != nil {
			return "", err
		}
		if err := w.WriteByte('\n'); err != nil {
			return "", err
		}
	}
	if err := w.Flush(); err != nil {
		return "", err
	}

	return tmpFile.Name(), nil
}

// --- 以下、マージ処理 (Min-Heap) ---

type MergeItem struct {
	Value     string
	ReaderIdx int
}

// StringHeap は MergeItem の最小ヒープを実装します。
type StringHeap []*MergeItem

func (h StringHeap) Len() int            { return len(h) }
func (h StringHeap) Less(i, j int) bool  { return h[i].Value < h[j].Value }
func (h StringHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *StringHeap) Push(x interface{}) { *h = append(*h, x.(*MergeItem)) }
func (h *StringHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[0 : n-1]
	return item
}

// mergeChunks は複数の一時ファイルをマージして出力ファイルに書き込みます。
func mergeChunks(files []*os.File, outputPath string) error {
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output: %w", err)
	}
	defer outputFile.Close()

	// writer.go で定義されている前提の newCRLFWriter を使用
	// もし writer.go がない場合は bufio.NewWriter(outputFile) に置き換えてください
	writer := newCRLFWriter(outputFile)

	scanners := make([]*bufio.Scanner, len(files))
	h := &StringHeap{}
	heap.Init(h)

	// 各ファイルの先頭行を読み込んでヒープへ
	for i, f := range files {
		scanners[i] = bufio.NewScanner(f)
		buf := make([]byte, 1024*1024)
		scanners[i].Buffer(buf, 10*1024*1024)

		if scanners[i].Scan() {
			heap.Push(h, &MergeItem{
				Value:     scanners[i].Text(),
				ReaderIdx: i,
			})
		}
	}

	// ヒープから最小値を取り出し続ける
	for h.Len() > 0 {
		minItem := heap.Pop(h).(*MergeItem)

		// 書き込み (newCRLFWriterがよしなに変換する想定)
		if _, err := writer.Write([]byte(minItem.Value)); err != nil {
			return err
		}
		if _, err := writer.Write([]byte("\n")); err != nil {
			return err
		}

		// 取り出したファイルから次を補充
		idx := minItem.ReaderIdx
		if scanners[idx].Scan() {
			heap.Push(h, &MergeItem{
				Value:     scanners[idx].Text(),
				ReaderIdx: idx,
			})
		} else if err := scanners[idx].Err(); err != nil {
			return fmt.Errorf("error reading temp file %d: %w", idx, err)
		}
	}

	return nil
}
