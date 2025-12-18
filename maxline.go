package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
)

// runMaxLine は指定されたファイルの最大行サイズを計測して標準出力に出力します。
func runMaxLine(inputPath string) error {
	file, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open input file: %w", err)
	}
	defer file.Close()

	maxSize, err := measureMaxLine(file)
	if err != nil {
		return err
	}

	fmt.Println(maxSize)
	return nil
}

// measureMaxLine はio.Readerからデータを読み込み、最も長い行のバイト数を返します。
// メモリ使用量は定数オーダー O(1) です。
func measureMaxLine(r io.Reader) (int64, error) {
	// 読み込みバッファ (例: 32KB)
	buf := make([]byte, 32*1024)

	var maxLineSize int64 = 0
	var currentLineSize int64 = 0

	for {
		n, err := r.Read(buf)
		if n > 0 {
			// バッファ内の処理
			// bytes.IndexByteを使って高速に \n を探す
			data := buf[:n]
			for {
				i := bytes.IndexByte(data, '\n')
				if i == -1 {
					// 改行が見つからない場合:
					// バッファの残りを全て現在の行サイズに加算して、次のReadへ
					currentLineSize += int64(len(data))
					break
				} else {
					// 改行が見つかった場合:
					// 改行までのバイト数を加算
					currentLineSize += int64(i) // \n は含めない長さがここに入る

					// ただし「行のバイトサイズ」には \n (1byte) も含める
					// また、CRLFの場合、直前の \r もiに含まれているので自然にカウントされる
					// ここでは "行の物理バイト数" なので改行コードも含めてカウントする
					currentLineSize += 1

					// 最大値の更新
					if currentLineSize > maxLineSize {
						maxLineSize = currentLineSize
					}

					// リセットして次へ
					currentLineSize = 0

					// 処理済み部分をスライスから除外
					data = data[i+1:]
				}
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, fmt.Errorf("error reading file: %w", err)
		}
	}

	// 最後の行に改行がない場合、EOF到達時のcurrentLineSizeも比較対象にする
	if currentLineSize > maxLineSize {
		maxLineSize = currentLineSize
	}

	return maxLineSize, nil
}
